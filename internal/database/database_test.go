package database

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func mustStartPostgresContainer() (func(context.Context, ...testcontainers.TerminateOption) error, error) {
	var (
		dbName = "database"
		dbPwd  = "password"
		dbUser = "user"
	)

	dbContainer, err := postgres.Run(
		context.Background(),
		"postgres:latest",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPwd),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	database = dbName
	password = dbPwd
	username = dbUser
	sslmode = "disable"
	schema = "public"

	dbHost, err := dbContainer.Host(context.Background())
	if err != nil {
		return dbContainer.Terminate, err
	}

	dbPort, err := dbContainer.MappedPort(context.Background(), "5432/tcp")
	if err != nil {
		return dbContainer.Terminate, err
	}

	host = dbHost
	port = dbPort.Port()

	return dbContainer.Terminate, err
}

func TestMain(m *testing.M) {
	teardown, err := mustStartPostgresContainer()
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	m.Run()

	if teardown != nil && teardown(context.Background()) != nil {
		log.Fatalf("could not teardown postgres container: %v", err)
	}
}

func TestNew(t *testing.T) {
	srv := New()
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestHealth(t *testing.T) {
	srv := New()

	stats := srv.Health()

	if stats["status"] != "up" {
		t.Fatalf("expected status to be up, got %s", stats["status"])
	}

	if _, ok := stats["error"]; ok {
		t.Fatalf("expected error not to be present")
	}

	if stats["message"] != "It's healthy" {
		t.Fatalf("expected message to be 'It's healthy', got %s", stats["message"])
	}

	// Verify connection stats are present (these come from pinging the database)
	if _, ok := stats["open_connections"]; !ok {
		t.Fatalf("expected open_connections to be present")
	}

	if _, ok := stats["in_use"]; !ok {
		t.Fatalf("expected in_use to be present")
	}

	if _, ok := stats["idle"]; !ok {
		t.Fatalf("expected idle to be present")
	}
}

type TestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:255"`
}

func TestDatabaseConnection(t *testing.T) {
	srv := New()

	// Get the underlying service to access the GORM DB
	s, ok := srv.(*service)
	if !ok {
		t.Fatal("expected srv to be of type *service")
	}

	// Test GORM AutoMigrate
	err := s.db.AutoMigrate(&TestModel{})
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}

	// Test GORM Create
	testRecord := TestModel{Name: "test"}
	result := s.db.Create(&testRecord)
	if result.Error != nil {
		t.Fatalf("failed to create record: %v", result.Error)
	}
	if testRecord.ID == 0 {
		t.Fatal("expected record ID to be set after creation")
	}

	// Test GORM Find
	var found TestModel
	result = s.db.First(&found, testRecord.ID)
	if result.Error != nil {
		t.Fatalf("failed to find record: %v", result.Error)
	}
	if found.Name != "test" {
		t.Fatalf("expected name to be 'test', got %s", found.Name)
	}

	// Test GORM Update
	result = s.db.Model(&found).Update("Name", "updated")
	if result.Error != nil {
		t.Fatalf("failed to update record: %v", result.Error)
	}

	// Verify update
	var updated TestModel
	s.db.First(&updated, testRecord.ID)
	if updated.Name != "updated" {
		t.Fatalf("expected name to be 'updated', got %s", updated.Name)
	}

	// Test GORM Delete
	result = s.db.Delete(&updated)
	if result.Error != nil {
		t.Fatalf("failed to delete record: %v", result.Error)
	}

	// Verify deletion
	var notFound TestModel
	result = s.db.First(&notFound, testRecord.ID)
	if result.Error == nil {
		t.Fatal("expected record to be deleted")
	}

	// Clean up - drop the test table
	s.db.Migrator().DropTable(&TestModel{})
}

func TestClose(t *testing.T) {
	srv := New()

	if srv.Close() != nil {
		t.Fatalf("expected Close() to return nil")
	}
}
