package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User representa un usuario en la base de datos.
type User struct {
	ID         primitive.ObjectID `bson:"_id"`
	Nombre     string             `bson:"nombre"`
	Correo     string             `bson:"correo"`
	Contrasena string             `bson:"contrasena"`
	Rol        string             `bson:"rol"`
}

// FileDocument representa un documento en la colección `imagenes.files`.
type FileDocument struct {
	ID          primitive.ObjectID `bson:"_id"`
	Filename    string             `bson:"filename"`
	Length      int64              `bson:"length"`
	ChunkSize   int                `bson:"chunkSize"`
	UploadDate  time.Time          `bson:"uploadDate"`
	EstudioID   string             `bson:"estudio_ID"`  // ID del estudio relacionado
	Anonimizada bool               `bson:"anonimizada"` // Indica si el archivo ha sido anonimizado
	Clave       string             `bson:"clave"`
}

type Diagnostico struct {
	Hallazgos     string    `bson:"hallazgos"`
	Impresion     string    `bson:"impresion"`
	Observaciones string    `bson:"observaciones"`
	Fecha         time.Time `bson:"fecha_Emision"`
	Medico        string    `bson:"realizo"`
}

type EstudioDocument struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	EstudioID   string             `bson:"estudio_ID"`
	Donador     string             `bson:"donador"`
	Hash        string             `bson:"hash"`
	Status      int                `bson:"status"`
	Diagnostico []Diagnostico      `bson:"diagnostico"`
}

type ImagenMetadata struct {
	NombreArchivo string      `json:"nombreArchivo"`
	Clave         string      `json:"clave"`
	Diagnostico   Diagnostico `json:"diagnostico"`
}
