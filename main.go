package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/handlers" // Importa el paquete de handlers
	"github.com/gorilla/mux"

	"webservice/Handl"
	"webservice/config"
	"webservice/middleware"
)

func main() {
	// Inicializar la base de datos y el cliente MongoDB
	db := config.InitializeDatabase()
	defer db.Close()

	client, database, bucket := config.InitializeMongoDBClient()
	defer client.Disconnect(context.Background())

	// Crear un enrutador y configurar rutas
	r := mux.NewRouter()

	// Definir las rutas
	r.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		Handl.RegisterHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		Handl.LoginHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/donacion", func(w http.ResponseWriter, r *http.Request) {
		Handl.UploadHandler(w, r, bucket, database)
	}).Methods("POST")

	r.HandleFunc("/image/{filename:[a-zA-Z0-9_\\-\\.]+}", func(w http.ResponseWriter, r *http.Request) {
		Handl.ImageHandler(w, r, bucket)
	}).Methods("GET")

	r.HandleFunc("/thumbnails", func(w http.ResponseWriter, r *http.Request) {
		Handl.ThumbnailHandler(w, r, client.Database("bdmdm"))
	}).Methods("GET")

	// Ruta para importar archivos y datos
	r.HandleFunc("/importar", func(w http.ResponseWriter, r *http.Request) {
		Handl.ImportarHandler(w, r, bucket, database)
	}).Methods("POST")

	// Ruta para generar diagnosticos de las imagenes
	r.HandleFunc("/diagnosticos/{id}", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Request recibido en /diagnosticos/{id}")
		Handl.UpdateDiagnosticoHandler(w, r, database)
	}).Methods("PUT")

	// Ruta para generar diagnosticos de las imagenes
	r.HandleFunc("/estudios/dicom", func(w http.ResponseWriter, r *http.Request) {
		Handl.FindEstudioIDByImagenNombreHandler(w, r, database)
	}).Methods("GET")

	// Aplicar el middleware de logging y CORS
	r.Use(middleware.LoggingMiddleware) // Aplica LoggingMiddleware a todas las rutas
	r.Use(middleware.CORSMiddleware)    // Aplica CORSMiddleware a todas las rutas

	// Configuración de CORS usando handlers.CORS
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),                      // Permite solicitudes desde cualquier ip
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS"}), // Permite los métodos necesarios
		handlers.AllowedHeaders([]string{"Content-Type"}),           // Permite los headers necesarios
	)(r)

	// Iniciar el servidor
	log.Println("Servidor escuchando en http://localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", corsHandler))
}
