package config

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/lib/pq"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ip string = "http://10.0.15.94:8080"

func GetIP() string {
	return ip
}

// InitializeDatabase configura la conexión a PostgreSQL.
func InitializeDatabase() *sql.DB {
	connStr := "user=postgres dbname=datalake password=123456789 sslmode=disable" // Reemplaza con tus credenciales
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error al conectar a la base de datos:", err)
	}
	return db
}

// InitializeMongoDBClient inicializa el cliente de MongoDB y el bucket de GridFS
func InitializeMongoDBClient() (*mongo.Client, *mongo.Database, *gridfs.Bucket) {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Asegúrate de que la conexión esté establecida
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	database := client.Database("bdmdm")

	// Crear un nuevo Bucket para GridFS, especificando la colección base "imagenes"
	bucket, err := gridfs.NewBucket(database, options.GridFSBucket().SetName("imagenes"))
	if err != nil {
		log.Fatal(err)
	}

	return client, database, bucket
}
