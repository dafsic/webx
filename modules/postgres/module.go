package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"

	"github.com/UrbanCompass/instrumentedsql"
	"github.com/lib/pq"
	ddsql "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	"gopkg.in/yaml.v2"

	"compass.com/app"
	"compass.com/app/flags"
	"compass.com/app/health"
	"compass.com/app/resources"
	"compass.com/app/secrets"
	datadog "compass.com/datadog_statsd"
)

const instrumentedPgDriverName = "instrumented-postgres"

var (
	log = app.Logger()
	// Default configuration values for the supported databases.
	databaseMappingFilename        = "postgres/database_mapping.yaml"
	databaseMappingResourceDefault = resources.Require(databaseMappingFilename)
)

type DbConf struct {
	Enabled           bool                                `yaml:"-"`
	Trace             bool                                `yaml:"-"`
	Hostname          string                              `yaml:"hostname"`
	Port              int32                               `yaml:"port"`
	DbName            string                              `yaml:"dbname"`
	SslMode           string                              `yaml:"sslmode"`
	UsernameSrc       flags.StringFromCLIOrSecretsManager `yaml:"-"`
	UsernameKey       string                              `yaml:"username"`
	Username          string                              `yaml:"-"`
	PasswordSrc       flags.StringFromCLIOrSecretsManager `yaml:"-"`
	PasswordKey       string                              `yaml:"password"`
	Password          string                              `yaml:"-"`
	DefaultSearchPath string                              `yaml:"default_searchpath"`
	MaxIdleConns      int                                 `yaml:"-"`
	MaxOpenConns      int                                 `yaml:"-"`
	MaxConnLifetime   time.Duration                       `yaml:"-"`
	DockerContainer   string                              `yaml:"-"`
	StatementTimeout  int                                 `yaml:"-"`
}

// ResolveSecrets uses secretsManager to resolve the secret members of a DbConf.
func (conf *DbConf) ResolveSecrets(secretsManager secrets.SecretsClient) error {
	password, err := conf.PasswordSrc.GetValue(secretsManager)
	if err != nil {
		return err
	}
	conf.Password = password
	username, err := conf.UsernameSrc.GetValue(secretsManager)
	if err != nil {
		return err
	}
	conf.Username = username
	return nil
}

// postgres.Module provides live connections to a set of Postgres Databases.
// It should be Installed to the app in the main, and then the application
// main can use the databases.
// Each database name that is used in DbNames must correspond to an entry
// in the databaseMappingFilename YAML.
// For each database that is configured, the postgres.Module will install
// command-line flags named --pg-<dbname>* that can be used to override
// the default config from the databaseMappingFilename YAML.
type Module struct {
	DbNames           []string
	DbMappingResource *resources.Resource
	nameToDbConf      map[string]*DbConf
	DataDog           *datadog.Module
	ddServiceName     string
}

func (m *Module) Priority() app.ModulePriority {
	return app.ModulePriorityHigh
}

// Load the defaults from the YAML file
func (m *Module) loadDefaults() (map[string]*DbConf, error) {
	r, err := m.DbMappingResource.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	dbConfDefaults := map[string]*DbConf{}
	err = yaml.Unmarshal(body, &dbConfDefaults)
	if err != nil {
		return nil, err
	}
	return dbConfDefaults, nil
}

// An instrumented sql.Logger that writes all queries to Log info.
type TraceLogger struct{}

func (l TraceLogger) Log(ctx context.Context, msg string, keyvals ...interface{}) {
	// TODO(ugo): Maybe this can be made much prettier for postgres only.
	log.Infof("TRACE: %s %v", msg, keyvals)
}

// dbConf returns the DbConf for a database that is configured in the YAML file.
// All the values are set according to the defaults.
func (m *Module) dbConf(dbName string) (*DbConf, error) {
	dbConfDefaults, err := m.loadDefaults()
	if err != nil {
		return nil, err
	}
	defaults := dbConfDefaults[dbName]
	if defaults == nil {
		return nil, fmt.Errorf("no defaults defined for database name '%s' in resource '%s'",
			dbName, m.DbMappingResource.Path())
	}
	result := &DbConf{}
	result.Enabled = true
	result.Trace = false
	result.Hostname = defaults.Hostname
	result.Port = defaults.Port
	result.DbName = defaults.DbName
	result.SslMode = defaults.SslMode
	if err = result.UsernameSrc.Set(defaults.UsernameKey); err != nil {
		return nil, err
	}
	if err = result.PasswordSrc.Set(defaults.PasswordKey); err != nil {
		return nil, err
	}
	result.DefaultSearchPath = defaults.DefaultSearchPath
	result.MaxIdleConns = 2
	result.MaxOpenConns = 50
	result.MaxConnLifetime = 30 * time.Minute
	result.StatementTimeout = 0
	return result, nil
}

func (m *Module) Configure(conf app.Configurator) error {
	if m.DbMappingResource == nil {
		m.DbMappingResource = databaseMappingResourceDefault
	}

	conf.Install(&datadog.Module{})

	sql.Register(instrumentedPgDriverName, instrumentedsql.WrapDriver(&pq.Driver{}, instrumentedsql.WithLogger(TraceLogger{})))

	conf.Install(&secrets.Module{})
	conf.Install(&health.Module{})
	dbConfDefaults, err := m.loadDefaults()
	if err != nil {
		return err
	}
	m.nameToDbConf = make(map[string]*DbConf)
	var validDbName = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)
	for _, dbName := range m.DbNames {
		if !validDbName.MatchString(dbName) {
			log.Fatalf("Invalid database name '%s'", dbName)
		}
		defaults := dbConfDefaults[dbName]
		if defaults == nil {
			log.Fatalf("No defaults defined for database name '%s' in resource '%s'",
				dbName, m.DbMappingResource.Path())
		}
		singleDbConf := DbConf{}
		m.nameToDbConf[dbName] = &singleDbConf
		conf.Flag("pg-"+dbName+"-enable", fmt.Sprintf("Enable access to the %s db.", dbName)).
			Default("true").
			BoolVar(&singleDbConf.Enabled)
		conf.Flag("pg-"+dbName+"-trace", fmt.Sprintf("Trace to log info queries sent to the %s db.", dbName)).
			Default("false").
			BoolVar(&singleDbConf.Trace)
		conf.Flag("pg-"+dbName+"-hostname", fmt.Sprintf("The hostname for the %s db.", dbName)).
			Default(defaults.Hostname).
			StringVar(&singleDbConf.Hostname)
		conf.Flag("pg-"+dbName+"-port", fmt.Sprintf("The port for the %s db.", dbName)).
			Default(strconv.Itoa(int(defaults.Port))).
			Int32Var(&singleDbConf.Port)
		conf.Flag("pg-"+dbName+"-dbname", fmt.Sprintf("The dbname for the %s db.", dbName)).
			Default(defaults.DbName).
			StringVar(&singleDbConf.DbName)
		conf.Flag("pg-"+dbName+"-sslmode", fmt.Sprintf("The sslmode for the %s db.", dbName)).
			Default(defaults.SslMode).
			StringVar(&singleDbConf.SslMode)
		conf.Flag("pg-"+dbName+"-username", fmt.Sprintf("The username for the %s db.", dbName)).
			Default(defaults.UsernameKey).
			SetValue(&singleDbConf.UsernameSrc)
		conf.Flag("pg-"+dbName+"-password", fmt.Sprintf("The password for the %s db.", dbName)).
			Default(defaults.PasswordKey).
			SetValue(&singleDbConf.PasswordSrc)
		conf.Flag("pg-"+dbName+"-default-searchpath", fmt.Sprintf("The default postgres searchpath for the %s db.", dbName)).
			Default(defaults.DefaultSearchPath).
			StringVar(&singleDbConf.DefaultSearchPath)
		conf.Flag("pg-"+dbName+"-max-idle-conns", fmt.Sprintf("The maximum number of idle connections for %s db.", dbName)).
			Default("2").
			IntVar(&singleDbConf.MaxIdleConns)
		conf.Flag("pg-"+dbName+"-max-open-conns", fmt.Sprintf("The maximum number of open connections for %s db (0 means unlimited).", dbName)).
			Default("50").
			IntVar(&singleDbConf.MaxOpenConns)
		conf.Flag("pg-"+dbName+"-max-conn-lifetime", fmt.Sprintf("The maximum amount of time a connection may be reused for %s db (0 means unlimited).", dbName)).
			Default("30m").
			DurationVar(&singleDbConf.MaxConnLifetime)
		conf.Flag("pg-"+dbName+"-docker-container", fmt.Sprintf("The name of the docker container that is running %s. Used for pg_dump", dbName)).
			Default("").
			StringVar(&singleDbConf.DockerContainer)
		conf.Flag("pg-"+dbName+"-statement-timeout", fmt.Sprintf("Abort any statement that takes more than the specified amount of time for the %s db (0 means no timeout, unit in millisecond).", dbName)).
			Default(strconv.Itoa(defaults.StatementTimeout)).
			IntVar(&singleDbConf.StatementTimeout)
	}

	conf.Flag("pg-dd-service-name", "Override datadog postgres service name. Default is {app.Name}.db").
		Default("").
		StringVar(&m.ddServiceName)
	return nil
}

func (m *Module) ProvideHealthCheckSequence(dbs *Databases) []*health.Check {
	names := dbs.Names()
	result := make([]*health.Check, 0, len(names))
	for _, name := range names {
		db := dbs.Get(name)
		if db.IsEnabled() {
			result = append(result, &health.Check{
				Name:  "Postgres-" + name,
				Check: db.Session().Ping,
			})
		}
	}
	return result
}

// DbConfigs is a map of DB name to DbConf.
type DbConfigs map[string]*DbConf

// ProvidePostgresConfigs provides the postgres configuration for all the databases
// that have been configured in this Module.
// This configuration is "fully resolved" in that any secret parameter will have been resolved.
//
// Applications that want to use sqlx (our most standard way of using postgres) should not use
// this, and should prefer injecting directly a *Databases (see ProvidePostgresDatabases).
func (m *Module) ProvidePostgresConfigs(secretsManager secrets.SecretsClient) (DbConfigs, error) {
	for _, conf := range m.nameToDbConf {
		if err := conf.ResolveSecrets(secretsManager); err != nil {
			return nil, err
		}
	}
	return m.nameToDbConf, nil
}

// ProvidePostgresDatabases provides an already-dialed Databases object.
func (m *Module) ProvidePostgresDatabases(secretsManager secrets.SecretsClient) (*Databases, error) {
	// TODO(ugo): maybe it would be nice to just inject a DbConfigs object here, so
	// as to not replicate the code from ProvidePostgresConfigs.
	for _, conf := range m.nameToDbConf {
		if err := conf.ResolveSecrets(secretsManager); err != nil {
			return nil, err
		}
	}
	result := &Databases{items: make(map[string]Database)}
	for n, c := range m.nameToDbConf {
		db, err := dialDatabase(n, c, m.DataDog.Enabled)
		if err != nil {
			return nil, err
		}
		if db != nil {
			result.items[n] = db
		}
	}
	return result, nil
}

func (m *Module) Start(dd *datadog.Module) {
	m.DataDog = dd
	if dd.Enabled {
		ddsql.Register(instrumentedPgDriverName, instrumentedsql.WrapDriver(&pq.Driver{}, instrumentedsql.WithLogger(TraceLogger{})))

		// This is the case where the command line arg is not set. We fall back to app.Name here (app.Name is not available in the Configure method)
		if m.ddServiceName == "" {
			m.ddServiceName = app.Name
		}
		// Current behavior is to default to postgres.db in the Register function
		// gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql/sql.go
		// If this flag is not set, we will default to postgres. Keeping the same behavior of appending ".db"
		ddsql.Register("postgres", &pq.Driver{}, ddsql.WithServiceName(fmt.Sprintf("%s.db", m.ddServiceName)))
	}
}

// LazyLoadedDatabases provides a way to connect to databases that are
// configured in the YAML file but are not part of Module.DbNames
// It is useful for example for programs that do not know in advance which
// databases they may connect to.
type LazyLoadedDatabases struct {
	m              *Module
	secretsManager secrets.SecretsClient
}

// ProvideLazyLoadedPostgresDatabases provides an instance of LazyLoadedDatabases.
func (m *Module) ProvideLazyLoadedPostgresDatabases(secretsManager secrets.SecretsClient) *LazyLoadedDatabases {
	return &LazyLoadedDatabases{
		m:              m,
		secretsManager: secretsManager,
	}
}

// DialOneDatabase dials a database that is configured in the YAML file but was
// not configured as part of Module.DbNames
// It uses the customizeConf func (if non nil) to modify in place the configuration
// i.e. to change the configuration values from their defaults.
// Calling this method multiple times will produce a new connection each time,
// clients are expected to cache those connections to avoid resource leakage.
func (dbs *LazyLoadedDatabases) DialOneDatabase(dbName string, customizeConf func(*DbConf)) (Database, error) {
	if _, ok := dbs.m.nameToDbConf[dbName]; ok {
		return nil, fmt.Errorf("Database %s is part of the postgres.Module DbNames (%v). Access it using the injected *Databases.",
			dbName, dbs.m.DbNames)
	}
	conf, err := dbs.m.dbConf(dbName)
	if err != nil {
		return nil, err
	}
	if customizeConf != nil {
		customizeConf(conf)
	}
	if err = conf.ResolveSecrets(dbs.secretsManager); err != nil {
		return nil, err
	}
	return dialDatabase(dbName, conf, dbs.m.DataDog.Enabled)
}
