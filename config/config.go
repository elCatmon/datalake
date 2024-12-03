package config

import (
	"context"
	"log"
	"time"

	_ "github.com/lib/pq"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ip string = "http://192.168.240.21:8080"
var POSTGRES_CONNECTION = "postgres://bdmdm:123456789@localhost:5432/bdmdm?sslmode=disable"
var SMTP_SERVER = "smtp.gmail.com"
var SMTP_PORT = "587"
var SMTP_EMAIL = "bdmdm.upp@gmail.com"
var SMTP_PASSWORD = "ildi heov pmce cjxu"

func GetIP() string {
	return ip
}
func GetPC() string {
	return POSTGRES_CONNECTION
}
func GetSServer() string {
	return SMTP_SERVER
}
func GetSP() string {
	return SMTP_PORT
}
func GetSMail() string {
	return SMTP_EMAIL
}
func GetSPD() string {
	return SMTP_PASSWORD
}

// InitializeMongoDBClient inicializa el cliente de MongoDB y el bucket de GridFS
func InitializeMongoDBClient() (*mongo.Client, *mongo.Database, *gridfs.Bucket) {
	// Configuración optimizada del cliente MongoDB
	clientOptions := options.Client().
		ApplyURI("mongodb://localhost:27017").
		SetMaxPoolSize(200).                 // Ajusta a 200 conexiones en el pool
		SetMinPoolSize(10).                  // Mínimo de conexiones
		SetMaxConnIdleTime(10 * time.Minute) // Tiempo máximo de inactividad de las conexiones
	// Intentar la conexión a MongoDB
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatalf("Error al conectar a MongoDB: %v", err)
	}
	// Verificar la conexión
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalf("Error al hacer ping a MongoDB: %v", err)
	}
	log.Println("Conexión a MongoDB establecida exitosamente.")
	// Obtener la base de datos y crear el bucket de GridFS
	database := client.Database("bdmdm")
	bucket, err := gridfs.NewBucket(database, options.GridFSBucket().SetName("imagenes"))
	if err != nil {
		log.Fatalf("Error al crear el bucket de GridFS: %v", err)
	}
	return client, database, bucket
}
