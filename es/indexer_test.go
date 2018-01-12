package es

import (
	"context"
    "errors"
	"testing"
    "crypto/sha1"
    "database/sql"
    "fmt"
    "io"
    "math/rand"
    "net"
    "net/url"
    "regexp"
    "strings"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var UID_REGEX = regexp.MustCompile("[a-zA-z0-9]{8}")

type TestDBManager struct {
    DB *sql.DB
    testDB string
}

// Move to more general utils.
const uidBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const lettersBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
func GenerateName(n int) string {
    b := make([]byte, n)
    b[0] = lettersBytes[rand.Intn(len(lettersBytes))]
    for i := range b[1:] {
        b[i+1] = uidBytes[rand.Intn(len(uidBytes))]
    }
    return string(b)
}

func (m *TestDBManager) InitTestDB() error {
    m.testDB = fmt.Sprintf("test_%s", strings.ToLower(GenerateName(10)))
    fmt.Println("Initializing test DB: ", m.testDB)

    // Open connection to RDBMS
    db, err := sql.Open("postgres", viper.GetString("test.mdb-url"))
    if err != nil {
        return err
    }

    fmt.Println("Opened MDB!")

    // Create a new temporary test database
    if _, err := db.Exec("CREATE DATABASE " + m.testDB); err != nil {
        return err
    }

    fmt.Printf("Created MDB %s\n", m.testDB)

    // Close first connection and connect to temp database
    db.Close()
    db, err = sql.Open("postgres", fmt.Sprintf(viper.GetString("test.url-template"), m.testDB))
    if err != nil {
        return err
    }

    // Run migrations
    err = m.importRemoteSchema(db)
    if err != nil {
        return err
    }

    // Setup SQLBoiler
    m.DB = db

    return nil
}

func (m *TestDBManager) DestroyTestDB() error {
    fmt.Println("Destroying testDB: ", m.testDB)

    // Close temp DB
    err := m.DB.Close()
    //err := boil.GetDB().(*sql.DB).Close()
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
    if err != nil {
        return err
    }

    return nil
}

// Supports:
// postgres://<host>/<dbname>?sslmode=disable&user=<user>&password=<password>"
// postgres://<user>:<password>@<host>/<dbname>?sslmode=disable"
// Returns host, dbname, user, password
func parseConnectionString(cs string) (string, string, string, string, error) {
    u, err := url.Parse(cs)
    if err != nil {
        return "", "", "", "", err
    }
    host, _, err := net.SplitHostPort(u.Host)
    if err != nil {
        host = u.Host
    }
    dbname := strings.TrimLeft(u.Path, "/")
    var user, password string
    if u.User != nil {
        user = u.User.Username()
        password, _ = u.User.Password()
    } else {
        m, _ := url.ParseQuery(u.RawQuery)
        if val, ok := m["user"]; ok {
            user = val[0]
        } else {
            return "", "", "", "", errors.New("User not found in connection string.")
        }
        if val, ok := m["password"]; ok {
            password = val[0]
        } else {
            return "", "", "", "", errors.New("Password not found in connection string.")
        }
    }

    return host, dbname, user, password, nil
}

// Copies latest database schema with constants to tmp test db.
func (m *TestDBManager) importRemoteSchema(testDB *sql.DB) error {
    host, dbname, user, password, err := parseConnectionString(viper.GetString("mdb.url"))
    if err != nil {
        return err
    }
    _, _, localUser, _, err := parseConnectionString(viper.GetString("test.mdb-url"))
    if err != nil {
        return err
    }
    viper.GetString("test.mdb-url")
    command := fmt.Sprintf(`
        CREATE EXTENSION postgres_fdw;
        CREATE SERVER s FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host '%s', dbname '%s');
        CREATE USER MAPPING FOR %s SERVER s OPTIONS (user '%s', password '%s');
        IMPORT FOREIGN SCHEMA public FROM SERVER s INTO public;
    `, host, dbname, localUser, user, password)
    fmt.Println(command)
    if _, err := testDB.Exec(command); err != nil {
        return err
    }
    return nil
}

func Sha1(s string) string {
    h := sha1.New()
    io.WriteString(h, s)
    return fmt.Sprintf("%x", h.Sum(nil))
}

func RandomSHA1() string {
    return Sha1(GenerateName(1024))
}


type IndexerSuite struct {
	suite.Suite
    TestDBManager
	esc *elastic.Client
    ctx context.Context
}

func (suite *IndexerSuite) SetupSuite() {
    fmt.Println("Setting up!")
	utils.InitConfig("", "../")
    err := suite.InitTestDB()
    if err != nil {
        panic(err)
    }
	suite.ctx = context.Background()

	// la := ESLogAdapter{T: suite.T()}
	// var err error
	// suite.esc, err = elastic.NewClient(
	// 	elastic.SetURL(viper.GetString("elasticsearch.url")),
	// 	elastic.SetSniff(false),
	// 	elastic.SetHealthcheckInterval(10*time.Second),
	// 	elastic.SetErrorLog(la),
	// 	elastic.SetInfoLog(la),
	// )
	// suite.Require().Nil(err)

    // Set package db and esc variables.
    fmt.Printf("set up suite.DB: %+v\n", suite.DB)
    InitWithDefault(suite.DB)
}

func (suite *IndexerSuite) TearDownSuite() {
    fmt.Printf("tear down suite.DB: %+v\n", suite.DB)
    Shutdown()
    suite.Require().Nil(suite.DestroyTestDB())
}

type ESLogAdapter struct{ *testing.T }

func (s ESLogAdapter) Printf(format string, v ...interface{}) { s.Logf(format, v...) }

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestIndexer(t *testing.T) {
	suite.Run(t, new(IndexerSuite))
}

func (suite *IndexerSuite) TestContentUnitsIndex() {
    indexer := MakeIndexer("test", []string{consts.ES_UNITS_INDEX})
    indexer.CreateIndexes()
    indexer.ReindexAll()
    // Do tests!
    res, err := esc.Search().Index("mdb_units_en").Do(suite.ctx)
    suite.Require().Nil(err)
    fmt.Printf("%+v\n", res)

    indexer.DeleteIndexes()
}
