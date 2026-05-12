package migrator

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/gorp.v1"
)

type MigratorTestSuite struct {
	suite.Suite

	Db    *sql.DB
	DbMap *gorp.DbMap
}

func TestMigratorTestSuite(t *testing.T) {
	suite.Run(t, new(MigratorTestSuite))
}

func (s *MigratorTestSuite) SetUpTest() {
	var err error
	db, err := sql.Open("sqlite3", ":memory:")
	require.Nil(s.T(), err)

	s.Db = db
	s.DbMap = &gorp.DbMap{Db: db, Dialect: &gorp.SqliteDialect{}}
}

func (s *MigratorTestSuite) TestCountPendingMigrations_WhenMigrationsPresent_ShouldReturnNumberOfPending() {
	// Arrange
	s.SetUpTest()
	// Act
	numberOfMigrationsPending, _ := countPendingMigrations(s.Db, "sqlite3", makeMemoryMigrations(), true)
	// Assert
	require.True(s.T(), numberOfMigrationsPending == 12)
}

func (s *MigratorTestSuite) TestCountPendingMigrations_WhenMigrationsApplied_ShouldApplyAndReturnNumberApplied() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	// Act
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	// Assert
	require.NoError(s.T(), err)
	require.Equal(s.T(), 12, numberOfMigrationsApplied)
	numberOfMigrationsPending, _ := countPendingMigrations(s.Db, "sqlite3", migrations, true)
	require.Equal(s.T(), 0, numberOfMigrationsPending)
}

func (s *MigratorTestSuite) TestAssertNoPendingMigrations_WhenMigrationsNotPending_ShouldNotError() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 12, numberOfMigrationsApplied)
	// Act
	err = assertNoPendingMigrations(s.Db, "sqlite3", migrations, true)
	// Assert
	require.NoError(s.T(), err)
}

func (s *MigratorTestSuite) TestMigrateUp_WhenNoTimeout_ShouldApplyAndReturnNumberApplied() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	serviceMigrator := makeServiceMigratorWithTimeout(0)
	// Act
	numberOfMigrationsApplied, err := serviceMigrator.MigrateUp(s.Db, "sqlite3", migrations)
	// Assert
	require.NoError(s.T(), err)
	require.Equal(s.T(), 12, numberOfMigrationsApplied)
	numberOfMigrationsPending, _ := countPendingMigrations(s.Db, "sqlite3", migrations, true)
	require.Equal(s.T(), 0, numberOfMigrationsPending)
}

func (s *MigratorTestSuite) TestMigrateUp_WhenTimeoutLongerThanExecution_ShouldApplyAndReturnNumberApplied() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	serviceMigrator := makeServiceMigratorWithTimeout(MaxDuration)
	// Act
	numberOfMigrationsApplied, err := serviceMigrator.MigrateUp(s.Db, "sqlite3", migrations)
	// Assert
	require.NoError(s.T(), err)
	require.Equal(s.T(), 12, numberOfMigrationsApplied)
	numberOfMigrationsPending, _ := countPendingMigrations(s.Db, "sqlite3", migrations, true)
	require.Equal(s.T(), 0, numberOfMigrationsPending)
}

func (s *MigratorTestSuite) TestAssertNoPendingMigrations_WhenMigrationsPending_ShouldError() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 12, numberOfMigrationsApplied)
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_add_middle_name.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	// Act
	err = assertNoPendingMigrations(s.Db, "sqlite3", migrations, true)
	// Assert
	require.Error(s.T(), err)
}

func (s *MigratorTestSuite) TestMigrationUp_WhenIgnoreUnknownWithUnknownInDb_ShouldSucceed() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_not_exist_in_repo_file.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 13, numberOfMigrationsApplied)

	serviceMigrator := makeServiceMigratorWithTimeout(MaxDuration)
	// Act
	numberOfMigrationsApplied, err = serviceMigrator.MigrateUp(s.Db, "sqlite3", makeMemoryMigrations())
	// Assert
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, numberOfMigrationsApplied)
}

func (s *MigratorTestSuite) TestMigrationUp_WhenNotIgnoreUnknownWithUnknownInDb_ShouldFail() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_not_exist_in_repo_file.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 13, numberOfMigrationsApplied)

	serviceMigrator := makeServiceMigratorWithTimeout(MaxDuration)
	serviceMigrator.ignoreUnknown = false
	// Act
	numberOfMigrationsApplied, err = serviceMigrator.MigrateUp(s.Db, "sqlite3", makeMemoryMigrations())
	// Assert
	require.Error(s.T(), err)
}

func (s *MigratorTestSuite) TestCountPendingMigrations_WhenIgnoreUnknownWithUnknownInDb_ShouldSucceed() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_not_exist_in_repo_file.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 13, numberOfMigrationsApplied)

	// Act
	numberOfPendingMigration, err := countPendingMigrations(s.Db, "sqlite3", makeMemoryMigrations(), true)
	// Assert
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, numberOfPendingMigration)
}

func (s *MigratorTestSuite) TestCountPendingMigrations_WhenNotIgnoreUnknownWithUnknownInDb_ShouldFail() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_not_exist_in_repo_file.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 13, numberOfMigrationsApplied)

	// Act
	_, err = countPendingMigrations(s.Db, "sqlite3", makeMemoryMigrations(), false)
	// Assert
	require.Error(s.T(), err)
}

func (s *MigratorTestSuite) TestAssertNoPendingMigrations_WhenIgnoreUnknownWithUnknownInDb_ShouldSucceed() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_not_exist_in_repo_file.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 13, numberOfMigrationsApplied)

	// Act
	err = assertNoPendingMigrations(s.Db, "sqlite3", makeMemoryMigrations(), true)
	// Assert
	require.NoError(s.T(), err)
}

func (s *MigratorTestSuite) TestAssertNoPendingMigrations_WhenNotIgnoreUnknownWithUnknownInDb_ShouldFail() {
	// Arrange
	s.SetUpTest()
	migrations := makeMemoryMigrations()
	migrations.Migrations = append(migrations.Migrations, &migrate.Migration{
		Id:   "11_not_exist_in_repo_file.sql",
		Up:   []string{"ALTER TABLE people ADD COLUMN middle_name text"},
		Down: []string{"ALTER TABLE people DROP COLUMN middle_name"},
	})
	numberOfMigrationsApplied, err := migrate.Exec(s.Db, "sqlite3", migrations, migrate.Up)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 13, numberOfMigrationsApplied)

	// Act
	err = assertNoPendingMigrations(s.Db, "sqlite3", makeMemoryMigrations(), false)
	// Assert
	require.Error(s.T(), err)
}

func makeServiceMigratorWithTimeout(timeout time.Duration) *ServiceMigrator {
	return &ServiceMigrator{
		runMigrations:   true,
		forceSchemaDump: false,
		runSchemaDump:   false,
		schemaPath:      "",
		schemasToDump:   nil,
		migratorTimeout: timeout,
		ignoreUnknown:   true,
	}
}

func makeMemoryMigrations() *migrate.MemoryMigrationSource {
	return &migrate.MemoryMigrationSource{
		Migrations: []*migrate.Migration{
			{
				Id:   "1_create_table.sql",
				Up:   []string{"CREATE TABLE people (id int)"},
				Down: []string{"DROP TABLE people"},
			},
			{
				Id:   "2_alter_table.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN first_name text"},
				Down: []string{"SELECT 0"}, // Not really supported
			},
			{
				Id:   "11_add_field_1.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_1 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_1"},
			},
			{
				Id:   "12_add_field_2.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_2 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_2"},
			},
			{
				Id:   "13_add_field_3.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_3 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_3"},
			},
			{
				Id:   "14_add_field_4.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_4 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_4"},
			},
			{
				Id:   "15_add_field_5.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_5 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_5"},
			},
			{
				Id:   "16_add_field_6.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_6 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_6"},
			},
			{
				Id:   "17_add_field_7.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_7 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_7"},
			},
			{
				Id:   "18_add_field_8.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_8 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_8"},
			},
			{
				Id:   "19_add_field_9.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_9 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_9"},
			},
			{
				Id:   "20_add_field_10.sql",
				Up:   []string{"ALTER TABLE people ADD COLUMN field_10 text"},
				Down: []string{"ALTER TABLE people DROP COLUMN field_10"},
			},
		},
	}
}
