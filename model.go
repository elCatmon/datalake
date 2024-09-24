package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User representa un usuario en la base de datos.
type User struct {
	ID         string `json:"usuario_id"`
	Nombre     string `json:"nombre"`
	Correo     string `json:"correo"`
	Contrasena string `json:"contrasena"`
}

// FileDocument representa un documento en la colección `imagenes.files`.
type FileDocument struct {
	ID         primitive.ObjectID `bson:"_id"`
	Filename   string             `bson:"filename"`
	Length     int64              `bson:"length"`
	ChunkSize  int                `bson:"chunkSize"`
	UploadDate time.Time          `bson:"uploadDate"`
}

type Imagen struct {
	Clave       string `bson:"clave"`
	Dicom       string `bson:"dicom"`
	Imagen      string `bson:"imagen"`
	Anonimizada bool   `bson:"anonimizada"`
}

type Diagnostico struct {
	Hallazgos     string `bson:"hallazgos"`
	Impresion     string `bson:"impresion"`
	Observaciones string `bson:"observaciones"`
}

type EstudioDocument struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	EstudioID       string             `bson:"estudio_ID"`
	Donador         string             `bson:"donador"`
	Hash            string             `bson:"hash"`
	Status          int                `bson:"status"`
	FechaNacimiento time.Time          `bson:"fecha_nacimiento"`
	FechaEstudio    time.Time          `bson:"fecha_estudio"`
	Imagenes        []Imagen           `bson:"imagenes"`
	Diagnostico     []Diagnostico      `bson:"diagnostico"`
}
