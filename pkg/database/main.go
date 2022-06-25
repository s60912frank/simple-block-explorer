package database

import (
	"fmt"
	"portto-explorer/pkg/model"
	"time"

	"github.com/jpillora/backoff"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DBOptions struct {
	User     string
	Password string
	Name     string
	Host     string
	Port     string
}

type Database struct {
	DB *gorm.DB
}

func New(dbOptions *DBOptions) *Database {
	if len(dbOptions.User) == 0 {
		panic("db user is empty")
	}
	if len(dbOptions.Password) == 0 {
		panic("db password is empty")
	}
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbOptions.Host, dbOptions.User, dbOptions.Password, dbOptions.Name, dbOptions.Port,
	)

	b := &backoff.Backoff{
		Factor: 1.5,
		Min:    1 * time.Second,
		Max:    32 * time.Second,
	}

	var db *gorm.DB
	var err error

	for {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			CreateBatchSize: 100,
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		})
		if err != nil {
			d := b.Duration()
			fmt.Printf("%s, reconnecting in %s", err, d)
			if d == b.Max {
				panic(err)
			}
			time.Sleep(d)

			continue
		}
		//connected
		b.Reset()
		break
	}

	// do auto migration
	db.AutoMigrate([]interface{}{
		&model.Block{},
		&model.Transaction{},
	}...)

	// if config := Config.GetConfig(); config.Settings.DebugMode {
	// 	d = db.Debug()
	// }
	return &Database{
		DB: db,
	}
}

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()

	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func (d *Database) Tx(callback func(tx *gorm.DB) error) error {
	return d.DB.Transaction(callback)
}
