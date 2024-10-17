package config

import (
	"context"
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

// InitializeMongoDBClient inicializa el cliente de MongoDB y el bucket de GridFS
func InitializeMongoDBClient() (*mongo.Client, *mongo.Database, *gridfs.Bucket) {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017").SetMaxPoolSize(10)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatalf("Error al conectar a MongoDB: %v", err)
	}
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalf("Error al hacer ping a MongoDB: %v", err)
	}
	log.Println("Conexión a MongoDB establecida exitosamente.")
	database := client.Database("bdmdm")
	bucket, err := gridfs.NewBucket(database, options.GridFSBucket().SetName("imagenes"))
	if err != nil {
		log.Fatalf("Error al crear el bucket de GridFS: %v", err)
	}

	return client, database, bucket
}
