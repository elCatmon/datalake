package main

import "go.mongodb.org/mongo-driver/bson/primitive"

// User representa un usuario en la base de datos.
type User struct {
	ID         int    `json:"id"`
	Nombre     string `json:"nombre"`
	Correo     string `json:"correo"`
	Contrasena string `json:"contrasena"`
}

// FileDocument representa un documento en la colección `imagenes.files`.
type FileDocument struct {
	ID       primitive.ObjectID `bson:"_id"`
	Filename string             `bson:"filename"`
}
