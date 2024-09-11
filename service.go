package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"golang.org/x/crypto/bcrypt"
)

// RegistrarUsuario agrega un nuevo usuario a la base de datos.
func RegistrarUsuario(db *sql.DB, user User) error {
	query := `INSERT INTO users (nombre, correo, contrasena) VALUES ($1, $2, $3)`

	// Ejecutar la consulta de inserción
	_, err := db.Exec(query, user.Nombre, user.Correo, user.Contrasena)
	if err != nil {
		return fmt.Errorf("error al registrar usuario: %v", err)
	}

	return nil
}

// ExisteCorreo verifica si un correo ya está registrado en la base de datos.
func ExisteCorreo(db *sql.DB, email string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM users WHERE correo=$1)"
	err := db.QueryRow(query, email).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ValidarUsuario verifica las credenciales del usuario y devuelve el ID del usuario si son válidas.
func ValidarUsuario(db *sql.DB, correo, contrasena string) (bool, string, error) {
	var id string
	var storedPassword string

	// Consulta para obtener la contraseña almacenada y el ID del usuario
	err := db.QueryRow("SELECT id, contrasena FROM users WHERE correo = $1", correo).Scan(&id, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			// Usuario no encontrado
			return false, "", nil
		}
		// Otro error
		return false, "", err
	}

	// Verificar la contraseña usando bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(contrasena))
	if err != nil {
		// Contraseña incorrecta
		return false, "", nil
	}

	return true, id, nil
}

// HashPassword genera el hash de una contraseña utilizando bcrypt.
func HashContraseña(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// EncontrarImagen busca una imagen en GridFS por nombre de archivo.
func EncontrarImagen(bucket *gridfs.Bucket, filename string) (*gridfs.DownloadStream, error) {
	downloadStream, err := bucket.OpenDownloadStreamByName(filename)
	if err != nil {
		return nil, err
	}
	return downloadStream, nil
}

func CrearFiltro(w http.ResponseWriter, tipoEstudio string, region string, edadMin string, edadMax string, sexo string) (bson.M, error) {
	// Crear el filtro de búsqueda para estudios
	filter := bson.M{
		"imagenes": bson.M{
			"$elemMatch": bson.M{
				"anonimizada": true,
			},
		},
		"status": "Aceptado",
	}

	// Agregar filtros opcionales a la consulta de estudios
	if tipoEstudio != "" {
		filter["estudio"] = tipoEstudio
	}
	if region != "" {
		filter["region"] = region
	}
	if edadMin != "" || edadMax != "" {
		edadFilter := bson.M{}
		if edadMin != "" {
			edadMinInt, err := strconv.Atoi(edadMin)
			if err != nil {
				http.Error(w, "Edad mínima inválida", http.StatusBadRequest)
			}
			edadFilter["$gte"] = edadMinInt
		}
		if edadMax != "" {
			edadMaxInt, err := strconv.Atoi(edadMax)
			if err != nil {
				http.Error(w, "Edad máxima inválida", http.StatusBadRequest)
			}
			edadFilter["$lte"] = edadMaxInt
		}
		filter["edad"] = edadFilter
	}
	if sexo != "" {
		filter["sexo"] = sexo
	}
	return filter, nil
}

func buscarEstudios(w http.ResponseWriter, studiesCollection *mongo.Collection, filter bson.M) ([]primitive.ObjectID, *mongo.Cursor, error) {

	// Buscar los estudios que cumplen con el filtro
	cursor, err := studiesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar estudios en la base de datos: "+err.Error(), http.StatusInternalServerError)
	}
	defer cursor.Close(context.Background())

	// Recolectar IDs de imágenes que cumplen con los criterios
	var imageIDs []primitive.ObjectID
	for cursor.Next(context.Background()) {
		var study EstudioDocument
		if err := cursor.Decode(&study); err != nil {
			http.Error(w, "Error al decodificar estudio: "+err.Error(), http.StatusInternalServerError)
		}

		for _, img := range study.Imagenes {
			if img.Anonimizada {
				imageID, err := primitive.ObjectIDFromHex(img.Imagen)
				if err != nil {
					http.Error(w, "Error al convertir ID de imagen: "+err.Error(), http.StatusInternalServerError)
				}
				imageIDs = append(imageIDs, imageID)
			}
		}
	}

	return imageIDs, cursor, nil
}

func BuscarImagenes(w http.ResponseWriter, filter bson.M, cursor *mongo.Cursor, imageIDs []primitive.ObjectID, db *mongo.Database) ([]string, error) {
	// Obtener la colección de archivos GridFS
	imagesCollection := db.Collection("imagenes.files")
	// Filtrar archivos con IDs en imageIDs y que terminen en .jpg
	filter = bson.M{
		"_id":      bson.M{"$in": imageIDs},
		"filename": bson.M{"$regex": `\.jpg$`},
	}
	// Buscar los archivos en la colección usando el filtro
	cursor, err := imagesCollection.Find(context.Background(), filter)
	if err != nil {
		http.Error(w, "Error al buscar archivos en la base de datos: "+err.Error(), http.StatusInternalServerError)
	}
	defer cursor.Close(context.Background())
	var images []string
	for cursor.Next(context.Background()) {
		var fileInfo FileDocument
		if err := cursor.Decode(&fileInfo); err != nil {
			http.Error(w, "Error al decodificar archivo: "+err.Error(), http.StatusInternalServerError)
		}
		// Obtener el nombre del archivo y construir la URL
		filename := fileInfo.Filename
		if filename != "" {
			imageURL := ip + "/image/" + filename
			images = append(images, imageURL)
		}
	}
	return images, nil
}
