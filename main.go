package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/handlers" // Importa el paquete de handlers
	"github.com/gorilla/mux"
)

func main() {
	// Inicializar la base de datos y el cliente MongoDB
	db := InitializeDatabase()
	defer db.Close()

	client, database, bucket := InitializeMongoDBClient()
	defer client.Disconnect(context.Background())

	// Crear un enrutador y configurar rutas
	r := mux.NewRouter()

	// Definir las rutas
	r.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		RegisterHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		LoginHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		UploadHandler(w, r, bucket)
	}).Methods("POST")

	r.HandleFunc("/image/{filename:[a-zA-Z0-9_\\-\\.]+}", func(w http.ResponseWriter, r *http.Request) {
		ImageHandler(w, r, bucket)
	}).Methods("GET")

	r.HandleFunc("/thumbnails", func(w http.ResponseWriter, r *http.Request) {
		ThumbnailHandler(w, r, client.Database("bdmdm"))
	}).Methods("GET")

	// Ruta para importar archivos y datos
	r.HandleFunc("/importar", func(w http.ResponseWriter, r *http.Request) {
		handleImportar(w, r, bucket, database)
	}).Methods("POST")

	// Configuración de CORS usando handlers.CORS
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),                      // Permite solicitudes desde localhost:3000
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}), // Permite los métodos necesarios
		handlers.AllowedHeaders([]string{"Content-Type"}),           // Permite los headers necesarios
	)(r)

	// Iniciar el servidor
	log.Println("Servidor escuchando en http://localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", corsHandler))
}
