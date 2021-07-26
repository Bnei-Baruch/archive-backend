package utils

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/migrations"
)

type TestDBManager struct {
	DB     *sql.DB
	myDB   *sql.DB
	testDB string
}

func (m *TestDBManager) InitTestMDB() error {
	db, err := m.initTestDB(viper.GetString("test.mdb-url"), viper.GetString("test.url-template"), "mdb")
	if err != nil {
		return err
	}
	m.DB = db
	// Run migrations
	return m.runMigrations(m.DB)
}

func (m *TestDBManager) InitTestMyDB() error {
	db, err := m.initTestDB(viper.GetString("test.mydb-url"), viper.GetString("test.mydb-url"), "mydb")
	if err != nil {
		return err
	}
	m.myDB = db
	// Run migrations
	return m.runMigrations(m.myDB)
}

func (m *TestDBManager) initTestDB(url, template, name string) (*sql.DB, error) {
	m.testDB = fmt.Sprintf("test_%s_%s", name, strings.ToLower(GenerateName(10)))

	// Open connection to RDBMS
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}

	// Create a new temporary test database
	if _, err := db.Exec("CREATE DATABASE " + m.testDB); err != nil {
		return nil, err
	}

	// Close first connection and connect to temp database
	db.Close()
	db, err = sql.Open("postgres", fmt.Sprintf(template, m.testDB))
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (m *TestDBManager) DestroyTestDB() error {
	// Close temp DB
	err := m.DB.Close()
	if err != nil {
		return err
	}

	// Connect to MDB
	db, err := sql.Open("postgres", viper.GetString("test.mdb-url"))
	if err != nil {
		return err
	}

	// Drop test DB
	_, err = db.Exec("DROP DATABASE " + m.testDB)
	return err
}

func (m *TestDBManager) runMigrations(db *sql.DB) error {
	var visit = func(path string, f os.FileInfo, err error) error {
		match, _ := regexp.MatchString(".*\\.sql$", path)
		if !match {
			return nil
		}

		//fmt.Printf("Applying migration %s\n", path)
		m, err := migrations.NewMigration(path)
		if err != nil {
			fmt.Printf("Error migrating %s, %s", path, err.Error())
			return err
		}

		for _, statement := range m.Up() {
			if _, err := db.Exec(statement); err != nil {
				return fmt.Errorf("Unable to apply migration %s: %s\nStatement: %s\n", m.Name, err, statement)
			}
		}

		return nil
	}

	return filepath.Walk("../migrations", visit)
}
