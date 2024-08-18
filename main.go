package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	ID          int
	Name        string
	Price       float64
	Description string
}

var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDIS_ADDR"),
	Password: os.Getenv("REDIS_PASSWORD"), // replace with your password
	DB:       0,
	TLSConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
	},
})

var db *gorm.DB

func init() {
	var err error
	dsn := "host=localhost user=root dbname=cache password=root"
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// Dropar a tabela (se existir)
	err = db.Migrator().DropTable(&Product{})
	if err != nil {
		fmt.Println("Error dropping table:", err)
	}
	db.AutoMigrate(&Product{})

	// Populate initial data
	products := []Product{
		{ID: 123, Name: "Banana", Price: 10.5, Description: "Banana from Brazil"},
		//{Name: "Maca", Price: 5.5, Description: "Maca from Argentina"},
	}
	db.Create(&products)
}

func getProductFromCache(productId int) (*Product, error) {
	val, err := rdb.Get(ctx, fmt.Sprintf("product:%d", productId)).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var product Product
	err = json.Unmarshal([]byte(val), &product)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func getProductFromDB(productId int) (*Product, error) {
	var product Product
	result := db.First(&product, productId)
	if result.Error != nil {
		return nil, result.Error
	}
	return &product, nil
}

func main() {
	product, err := getProductFromCache(123)
	if err != nil {
		fmt.Println("Error fetching from cache:", err)
	} else if product == nil {
		// Produto não encontrado no cache, buscar no banco de dados
		product, err = getProductFromDB(123)
		if err != nil {
			fmt.Println("Error fetching from database:", err)
			return
		}

		// Armazenar o produto no cache
		productJSON, err := json.Marshal(product)
		if err != nil {
			fmt.Println("Error marshalling product:", err)
			return
		}
		err = rdb.Set(ctx, fmt.Sprintf("product:%d", product.ID), productJSON, time.Minute*10).Err()
		if err != nil {
			fmt.Println("Error setting cache:", err)
		}
	}

	fmt.Println("Product:", product)
}
