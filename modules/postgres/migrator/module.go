package migrator

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rubenv/sql-migrate"

	"compass.com/app"
	"compass.com/app/resources"

	"compass.com/postgres"
	"compass.com/postgres/pgdumpschema"
)

const (
	MinDuration time.Duration = 1
	MaxDuration time.Duration = 1<<63 - 1
)

var (
	log = app.Logger()
)

type Module struct {
	RunMigrations   bool `help:"Run SQL migrations at startup."`
	Limit           int
	MigrateOnly     bool `help:"Run app in migration mode."`
	MigratorTimeout int  `help:"Timeout to terminate the migration process (seconds).  Use 0 for no timeout." default:"0"`

	MigratorDumpSchemaEnv   string `help:"Dump db schema when running in given environment" default:"default"`
	MigratorForceDumpSchema bool   `help:"Dump DB schema, regardless of environment or whether migrations were applied. Useful for development."`
	SchemaPath              string
	SchemasToDump           []string
	CheckUnknown            bool
}

func (mm *Module) Configure(conf app.Configurator) error {
	upCmd := conf.Command("up", "Migrate the db to a more recent version.")
	upCmd.Arg("limit", "The max number of migrations to apply.").Default("0").IntVar(&mm.Limit)
	upCmd.Default()
	downCmd := conf.Command("down", "Rollback the db to a less recent version.")
	downCmd.Arg("limit", "The max number of migrations to rollback.").Default("1").IntVar(&mm.Limit)
	conf.Flag("check-unknown-migrations", "check if database contain migrations that relate to files that are not in the current code branch").
		Default("false").
		BoolVar(&mm.CheckUnknown)
	return nil
}

func (mm *Module) ProvideMigrator(command app.SelectedCommand) Migrator {
	if mm.MigrateOnly {
		return &CmdMigrator{
			limit:         mm.Limit,
			cmd:           command,
			runSchemaDump: app.Env == mm.MigratorDumpSchemaEnv,
			schemaPath:    mm.SchemaPath,
			schemasToDump: mm.SchemasToDump,
			ignoreUnknown: !mm.CheckUnknown,
		}
	}
	return &ServiceMigrator{
		runMigrations:   mm.RunMigrations,
		forceSchemaDump: mm.MigratorForceDumpSchema,
		runSchemaDump:   app.Env == mm.MigratorDumpSchemaEnv,
		schemaPath:      mm.SchemaPath,
		schemasToDump:   mm.SchemasToDump,
		migratorTimeout: time.Duration(mm.MigratorTimeout) * time.Second,
		ignoreUnknown:   !mm.CheckUnknown,
	}
}

type ResourceMigrationSource struct {
	sql []*resources.Resource
}

func (rmm *ResourceMigrationSource) FindMigrations() ([]*migrate.Migration, error) {
	migrations := make([]*migrate.Migration, len(rmm.sql))
	for i, s := range rmm.sql {
		migration, err := resourceToMigration(s)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing migration %s", s.Path())
		}
		migrations[i] = migration
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Id < migrations[j].Id
	})
	return migrations, nil
}

func resourceToMigration(resource *resources.Resource) (*migrate.Migration, error) {
	r, err := resource.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	body, err := ioutil.ReadAll(r)
	return migrate.ParseMigration(filepath.Base(resource.Path()), bytes.NewReader(body))
}

type Migrator interface {
	AssertNoPendingMigrations(db postgres.Database, sql []*resources.Resource) error
	CountPendingMigrations(db postgres.Database, sql []*resources.Resource) (int, error)
	Migrate(db postgres.Database, sql []*resources.Resource) error
}

type ServiceMigrator struct {
	runMigrations   bool
	runSchemaDump   bool
	forceSchemaDump bool
	schemaPath      string
	schemasToDump   []string
	migratorTimeout time.Duration
	ignoreUnknown   bool
}

func (m *ServiceMigrator) AssertNoPendingMigrations(db postgres.Database, sql []*resources.Resource) error {
	return assertNoPendingMigrations(db.Session().DB, "postgres", &ResourceMigrationSource{sql: sql}, m.ignoreUnknown)
}

func (m *ServiceMigrator) CountPendingMigrations(db postgres.Database, sql []*resources.Resource) (int, error) {
	return countPendingMigrations(db.Session().DB, "postgres", &ResourceMigrationSource{sql: sql}, m.ignoreUnknown)
}

func (m *ServiceMigrator) Migrate(db postgres.Database, sql []*resources.Resource) error {
	if !m.runMigrations {
		log.Info("migrations not applied (use --run-migrations to apply)")
		if m.forceSchemaDump {
			err := m.dumpSchema(db)
			if err != nil {
				return err
			}
		}
		return nil
	}

	migrationSource := &ResourceMigrationSource{sql: sql}
	migrationsApplied, err := m.MigrateUp(db.Session().DB, "postgres", migrationSource)
	if err != nil {
		return err
	}

	if m.forceSchemaDump || (m.runSchemaDump && len(m.schemasToDump) > 0 && migrationsApplied > 0) {
		err := m.dumpSchema(db)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ServiceMigrator) MigrateUp(db *sql.DB, dialect string, migrationSource migrate.MigrationSource) (migrationsApplied int, err error) {
	ch := make(chan error, 1)

	go func() {
		migrationsApplied, err = migrateUp(db, dialect, migrationSource, 0, m.ignoreUnknown)
		if err != nil {
			ch <- err
		} else {
			log.Info("Migration completed")
			ch <- nil
		}
	}()

	timeout := m.migratorTimeout
	if m.migratorTimeout == 0 {
		timeout = MaxDuration
	}
	select {
	case err := <-ch:
		if err != nil {
			err = errors.Wrapf(err, "Migrator error")
			log.Error(err)
			return 0, err
		}
	case <-time.After(timeout):
		err = errors.Errorf(`Migration taking longer than configured timeout of %v seconds.
If the migration is creating an index it could take a relatively long time
and this timeout flag should be increased accordingly or disabled when running this migration
(and then reset back to its normal value after the migration/cronjob is complete).
If there is a serious issue during the migration and it is determined that it is best to
cancel it, please consider connecting to the db and running SELECT pg_cancel_backend(pid)
to terminate the migration query`, m.migratorTimeout)
		log.Error(err)
		return 0, err
	}

	return migrationsApplied, nil
}

func (m *ServiceMigrator) dumpSchema(db postgres.Database) error {
	return dumpSchema(db, m.schemaPath, m.schemasToDump)
}

type CmdMigrator struct {
	limit         int
	cmd           app.SelectedCommand
	runSchemaDump bool
	schemaPath    string
	schemasToDump []string
	ignoreUnknown bool
}

func (m *CmdMigrator) AssertNoPendingMigrations(db postgres.Database, sql []*resources.Resource) error {
	return assertNoPendingMigrations(db.Session().DB, "postgres", &ResourceMigrationSource{sql: sql}, m.ignoreUnknown)
}

func (m *CmdMigrator) CountPendingMigrations(db postgres.Database, sql []*resources.Resource) (int, error) {
	return countPendingMigrations(db.Session().DB, "postgres", &ResourceMigrationSource{sql: sql}, m.ignoreUnknown)
}

func (m *CmdMigrator) Migrate(db postgres.Database, sql []*resources.Resource) error {
	migrationSource := &ResourceMigrationSource{sql: sql}
	var migrationsApplied int
	var err error
	switch m.cmd {
	case "up":
		migrationsApplied, err = migrateUp(db.Session().DB, "postgres", migrationSource, m.limit, m.ignoreUnknown)
	case "down":
		migrationsApplied, err = migrateDown(db.Session().DB, "postgres", migrationSource, m.limit, m.ignoreUnknown)
	default:
		panic("unsupported command")
	}
	if m.runSchemaDump && len(m.schemasToDump) > 0 && migrationsApplied > 0 {
		err := m.dumpSchema(db)
		if err != nil {
			return err
		}
	}
	if err == nil {
		os.Exit(0)
	}
	return err
}

func (m *CmdMigrator) dumpSchema(db postgres.Database) error {
	return dumpSchema(db, m.schemaPath, m.schemasToDump)
}

// Determine the number of migrations that were not yet executed successfully against the database.
func countPendingMigrations(db *sql.DB, dialect string, source migrate.MigrationSource, ignoreUnknown bool) (int, error) {
	migrate.SetIgnoreUnknown(ignoreUnknown)
	migrations, _, err := migrate.PlanMigration(db, dialect, source, migrate.Up, 0)
	if err != nil {
		return 0, errors.Wrap(err, "migrator could not count pending migrations")
	}
	return len(migrations), nil
}

// Assert that there are no pending migrations to be run against the database.
func assertNoPendingMigrations(db *sql.DB, dialect string, source migrate.MigrationSource, ignoreUnknown bool) error {
	countOfPendingMigrations, err := countPendingMigrations(db, dialect, source, ignoreUnknown)
	if err != nil {
		return err
	}
	if countOfPendingMigrations > 0 {
		return fmt.Errorf("more than one pending migration: %v", countOfPendingMigrations)
	}
	return nil
}

// The limit parameter controls the number of migrations to apply. If
// it is set to 0 then all migrations are applied. Return number of
// migrations applied.
func migrateUp(db *sql.DB, dialect string, source migrate.MigrationSource, limit int, ignoreUnknown bool) (int, error) {
	migrate.SetIgnoreUnknown(ignoreUnknown)
	n, err := migrate.ExecMax(db, dialect, source, migrate.Up, limit)
	if err != nil {
		return 0, errors.Wrap(err, "migrator could not complete migration")
	}
	log.Infof("Applied %d migrations", n)
	return n, nil
}

// The limit parameter controls the number of migrations to roll back.
// If it is set to 0 then all migrations are rolled back. Return
// number of migrations applied.
func migrateDown(db *sql.DB, dialect string, source migrate.MigrationSource, limit int, ignoreUnknown bool) (int, error) {
	migrate.SetIgnoreUnknown(ignoreUnknown)
	n, err := migrate.ExecMax(db, dialect, source, migrate.Down, limit)
	if err != nil {
		return 0, errors.Wrap(err, "migrator could not complete migration")
	}
	log.Infof("Rolled back %d migrations", n)
	return n, nil
}

func dumpSchema(db postgres.Database, schemaPath string, schemasToDump []string) error {
	outputFile, err := os.Create(schemaPath)
	if err != nil {
		log.Errorf("error creating schema dump file: %v", err)
		return err
	}
	defer outputFile.Close()

	dbConfig := db.Config()
	dumpConfig := &pgdumpschema.Config{
		Database: dbConfig.DbName,
		Host:     dbConfig.Hostname,
		Password: dbConfig.Password,
		Port:     int(dbConfig.Port),
		Schemas:  schemasToDump,
		Username: dbConfig.Username,
		Writer:   outputFile,
	}

	if dbConfig.DockerContainer != "" {
		dumpConfig.PGDump = fmt.Sprintf("docker exec %s pg_dump", dbConfig.DockerContainer)
		// Assumes that postgres is running on port 5432 within the container
		dumpConfig.Port = 5432
	}

	log.Infof("dumping schemas %s to %s (host=%s:%d, db=%s, user=%s)",
		strings.Join(schemasToDump, ", "), schemaPath, dbConfig.Hostname,
		dbConfig.Port, dbConfig.DbName, dbConfig.Username)
	err = pgdumpschema.Dump(dumpConfig)
	if err != nil {
		log.Errorf("error dumping schema: %v", err)
		return err
	}
	return nil
}
