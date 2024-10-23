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

// Estructura para la solicitud de cambio de contraseña
type ChangePasswordRequest struct {
	Email           string `json:"email"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
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
	Imagenes    []Imagen           `bson:"imagenes"`
	Diagnostico []Diagnostico      `bson:"diagnostico"`
}

type ImagenMetadata struct {
	NombreArchivo string              `json:"nombreArchivo"`
	Clave         string              `json:"clave"`
	Diagnostico   DiagnosticoMetadata `json:"diagnostico"`
}

// Estructura sin el campo "realizo" (Medico)
type DiagnosticoMetadata struct {
	Hallazgos     string    `bson:"hallazgos"`
	Impresion     string    `bson:"impresion"`
	Observaciones string    `bson:"observaciones"`
	Fecha         time.Time `bson:"fecha_Emision"`
}
