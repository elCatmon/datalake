package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers" // Importa el paquete de handlers
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"webservice/Handl"
	"webservice/config"
	"webservice/middleware"
	"webservice/services"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error cargando el archivo .env: %v", err)
	}

	// Inicializar la base de datos y el cliente MongoDB
	client, database, bucket := config.InitializeMongoDBClient()
	defer func() {
		log.Println("Desconectando de MongoDB...")
		if err := client.Disconnect(context.TODO()); err != nil {
			log.Fatalf("Error al desconectar de MongoDB: %v", err)
		}
		log.Println("Desconexión de MongoDB exitosa.")
	}()

	connStr := os.Getenv("POSTGRES_CONNECTION")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error al conectar con la base de datos: %v", err)
	}
	defer db.Close()

	// Crear un enrutador y configurar rutas
	r := mux.NewRouter()

	// Definir las rutas
	r.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		Handl.RegisterHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		Handl.LoginHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/verificarcorreo", func(w http.ResponseWriter, r *http.Request) {
		Handl.VerificarCorreoHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/cambiocontrasena", func(w http.ResponseWriter, r *http.Request) {
		Handl.CambiarContrasenaHandler(w, r, db)
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
		Handl.ActualizarDiagnosticoHandler(w, r, database)
	}).Methods("PATCH")

	// Ruta para buscar el estudio de una imagen dicom
	r.HandleFunc("/estudios/dicom", func(w http.ResponseWriter, r *http.Request) {
		Handl.BuscarEstudioIDImagenNombreHandler(w, r, database)
	}).Methods("GET")

	// Ruta para obtener el diagnostico de una imagen
	r.HandleFunc("/estudios/diagnostico", func(w http.ResponseWriter, r *http.Request) {
		Handl.GetDiagnosticoHandler(w, r, database)
	}).Methods("GET")

	// Configurar las rutas
	r.HandleFunc("/dataset/descarga", func(w http.ResponseWriter, r *http.Request) {
		// Aquí se llamará al manejador
		Handl.DatasetHandler(w, r, bucket, database)
	}).Methods("GET")

	r.HandleFunc("/dataset/predeterminado", func(w http.ResponseWriter, r *http.Request) {
		// Aquí se llamará al manejador
		Handl.DatasetPredeterminadoHandler(w, r)
	}).Methods("GET")

	r.HandleFunc("/api/estudios", func(w http.ResponseWriter, r *http.Request) {
		services.CreateEstudioHandler(w, r, db)
	}).Methods("POST")

	r.HandleFunc("/api/estudios/consulta", func(w http.ResponseWriter, r *http.Request) {
		services.GetEstudiosHandler(w, r, db)
	}).Methods("GET")

	r.HandleFunc("/api/estudios/confirmar-digitalizacion", func(w http.ResponseWriter, r *http.Request) {
		services.ConfirmarDigitalizacionHandler(w, r)
	}).Methods("POST")

	// Aplicar el middleware de logging y CORS
	r.Use(middleware.LoggingMiddleware) // Aplica LoggingMiddleware a todas las rutas
	r.Use(middleware.CORSMiddleware)    // Aplica CORSMiddleware a todas las rutas

	// Configuración de CORS usando handlers.CORS
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),                                                // Permite solicitudes desde cualquier origen
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"}), // Incluye PATCH
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),                    // Incluye Authorization si lo necesitas
	)(r)

	// Crear la instancia del servidor HTTP con configuración adicional
	server := &http.Server{
		Addr:           ":8080",     // Ajusta el puerto
		Handler:        corsHandler, // Handler configurado con CORS
		MaxHeaderBytes: 1 << 30,     // 1 GB para encabezados
	}

	// Iniciar el servidor HTTP con configuración personalizada
	log.Println("Servidor escuchando en http://localhost:8080...")
	log.Fatal(server.ListenAndServe()) // Usa ListenAndServe del servidor configurado
}
