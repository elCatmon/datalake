package services

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	_ "github.com/lib/pq"
	"gopkg.in/gomail.v2"
)

// Estructura para representar un estudio
type DetalleEstudio struct {
	TipoEstudio      string `json:"tipoEstudio"`
	CantidadImagenes int    `json:"cantidadImagenes"`
	EsDonacion       bool   `json:"esDonacion"`
	Observaciones    string `json:"observaciones"`
	Id               int    `json:"id"`
}

type Estudio struct {
	Folio            string           `json:"folio"`
	FechaRecepcion   time.Time        `json:"fechaRecepcion"`
	FechaDevolucion  *time.Time       `json:"fechaDevolucion,omitempty"`
	Correo           string           `json:"correo"`
	CURP             string           `json:"curp"`
	Carrera          string           `json:"carrera"`
	Cuatrimestre     string           `json:"cuatrimestre"`
	Area             string           `json:"area"`
	DetallesEstudios []DetalleEstudio `json:"detallesEstudios"`
}

type RequestData struct {
	Correo string `json:"correo"`
	Fecha  string `json:"fecha"`
	Folio  string `json:"folio"`
}

// Función para guardar un nuevo estudio en la base de datos
func CreateEstudio(estudio Estudio, db *sql.DB) error {
	// Iniciar una transacción
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error al iniciar la transacción: %v", err)
		return err
	}

	// Inserción en la tabla `estudios`
	queryEstudio := `INSERT INTO estudios (folio, fecha_recepcion, fecha_devolucion, correo, curp, carrera, cuatrimestre, area)
                     VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(queryEstudio,
		estudio.Folio,
		estudio.FechaRecepcion,
		estudio.FechaDevolucion, // Puntero para manejar el NULL
		estudio.Correo,
		estudio.CURP,
		estudio.Carrera,
		estudio.Cuatrimestre,
		estudio.Area,
	)
	if err != nil {
		log.Printf("Error al guardar el estudio en la tabla 'estudios': %v", err)
		tx.Rollback() // Revertir la transacción en caso de error
		return err
	}

	// Inserción de detalles en la tabla `detalles_estudios`
	queryDetalle := `INSERT INTO detalles_estudios (folio, tipo_estudio, cantidad_imagenes, es_donacion, observaciones)
                     VALUES ($1, $2, $3, $4, $5)`

	for _, detalle := range estudio.DetallesEstudios {
		_, err := tx.Exec(queryDetalle,
			estudio.Folio,
			detalle.TipoEstudio,
			detalle.CantidadImagenes,
			detalle.EsDonacion,
			detalle.Observaciones,
		)

		if err != nil {
			log.Printf("Error al guardar detalle del estudio en la tabla 'detalles_estudios': %v", err)
			tx.Rollback() // Revertir la transacción en caso de error
			return err
		}
	}

	// Confirmar la transacción si no hubo errores
	if err := tx.Commit(); err != nil {
		log.Printf("Error al confirmar la transacción: %v", err)
		return err
	}

	return nil // Retornar nil si no hay errores
}

// Función para generar un PDF con los detalles del estudio
func GeneraPDF(estudio Estudio) (*bytes.Buffer, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.ImageOptions("./images/logo_bdmdm.png", 12, 10, 50, 15, false, gofpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")
	pdf.ImageOptions("./images/logo_upp.png", 97, 10, 12, 15, false, gofpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")
	pdf.ImageOptions("./images/logo_citedi.png", 148, 10, 50, 15, false, gofpdf.ImageOptions{ImageType: "png", ReadDpi: true}, 0, "")

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(50, 50, "Repositorio")

	pdf.SetY(45)

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Informacion de Registro")
	pdf.Ln(12)

	// Información general del registro
	pdf.SetFont("Arial", "", 12)

	// Folio
	if estudio.Folio != "" {
		pdf.Cell(0, 10, fmt.Sprintf("Folio: %s", estudio.Folio))
		pdf.Ln(8)
	}

	// Fecha de Recepcion
	if !estudio.FechaRecepcion.IsZero() {
		pdf.Cell(0, 10, fmt.Sprintf("Fecha de Recepcion: %s", estudio.FechaRecepcion.Format("02-01-2006")))
		pdf.Ln(8)
	}

	// Fecha de Devolucion (solo si no es nil)
	if estudio.FechaDevolucion != nil {
		pdf.Cell(0, 10, fmt.Sprintf("Fecha de Devolucion: %s", estudio.FechaDevolucion.Format("02-01-2006")))
		pdf.Ln(8)
	}

	// Correo
	if estudio.Correo != "" {
		pdf.Cell(0, 10, fmt.Sprintf("Correo: %s", estudio.Correo))
		pdf.Ln(8)
	}

	// CURP
	if estudio.CURP != "" {
		pdf.Cell(0, 10, fmt.Sprintf("CURP: %s", estudio.CURP))
		pdf.Ln(8)
	}

	// Carrera
	if estudio.Carrera != "" {
		pdf.Cell(0, 10, fmt.Sprintf("Carrera: %s", estudio.Carrera))
		pdf.Ln(8)
	}

	// Cuatrimestre
	if estudio.Cuatrimestre != "" {
		pdf.Cell(0, 10, fmt.Sprintf("Cuatrimestre: %s", estudio.Cuatrimestre))
		pdf.Ln(8)
	}

	// Area
	if estudio.Area != "" {
		pdf.Cell(0, 10, fmt.Sprintf("Area: %s", estudio.Area))
		pdf.Ln(12)
	}

	// Información de cada detalle de estudio
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 10, "Detalles de los Estudios")
	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)

	for idx, detalle := range estudio.DetallesEstudios {
		pdf.Cell(0, 10, fmt.Sprintf("Estudio #%d", idx+1))
		pdf.Ln(8)

		// Tipo de Estudio
		if detalle.TipoEstudio != "" {
			pdf.Cell(0, 10, fmt.Sprintf("  Tipo de Estudio: %s", detalle.TipoEstudio))
			pdf.Ln(8)
		}

		// Cantidad de Imagenes
		if detalle.CantidadImagenes > 0 {
			pdf.Cell(0, 10, fmt.Sprintf("  Cantidad de Imagenes: %d", detalle.CantidadImagenes))
			pdf.Ln(8)
		}

		// Es Donacion
		donacion := "No"
		if detalle.EsDonacion {
			donacion = "Si"
		}
		pdf.Cell(0, 10, fmt.Sprintf("  Donacion: %s", donacion))
		pdf.Ln(8)

		// Observaciones
		if detalle.Observaciones != "" {
			pdf.Cell(0, 10, fmt.Sprintf("  Observaciones: %s", detalle.Observaciones))
			pdf.Ln(12)
		}
	}

	// Generar PDF en buffer
	var pdfBuffer bytes.Buffer
	if err := pdf.Output(&pdfBuffer); err != nil {
		return nil, err
	}

	return &pdfBuffer, nil
}

// Función para enviar el correo electrónico con el PDF adjunto
func EnviaCorreoPDF(estudio Estudio, pdfBuffer *bytes.Buffer) error {
	// Crear un nuevo mensaje de correo
	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_EMAIL")) // Email remitente desde el entorno
	m.SetHeader("To", estudio.Correo)            // Email destinatario
	m.SetHeader("Subject", "Registro de donación")
	m.SetBody("text/plain", "Adjunto encontrarás el PDF con los detalles de tu registro.")
	m.Attach("registro.pdf", gomail.SetCopyFunc(func(w io.Writer) error {
		_, err := pdfBuffer.WriteTo(w)
		return err
	}))

	// Leer la configuración del servidor de correo desde las variables de entorno
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpEmail := os.Getenv("SMTP_EMAIL")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Convertir el puerto SMTP de string a int
	port, err := strconv.Atoi(smtpPort)
	if err != nil {
		return fmt.Errorf("error al convertir el puerto SMTP: %v", err)
	}

	// Configuración del servidor de correo
	d := gomail.NewDialer(smtpServer, port, smtpEmail, smtpPassword)

	// Enviar el correo
	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("error al enviar el correo: %v", err)
	}

	return nil
}

func EnviaCorreoConfirmacion(correo string, fecha string, folio string) error {
	// Formatear la fecha
	parsedTime, err := time.Parse(time.RFC3339, fecha)
	if err != nil {
		fmt.Println("Error al parsear la fecha:", err)
	}
	// Formatear la fecha al formato dd-mm-aaaa
	fechaFormateada := parsedTime.Format("02-01-2006")
	fmt.Println("Fecha formateada:", fechaFormateada)

	// Crear el mensaje con formato HTML
	mensaje := fmt.Sprintf(`
    Buen día<br><br>
    Le informamos que el proceso de digitalización de sus estudios prestados el día: <strong>%s</strong> 
    con el número de folio: <strong>%s</strong>, ha sido realizado exitosamente. Por lo tanto, ya puede pasar 
    a recogerlos en cualquier momento.<br><br>
    Atentamente<br>
    El equipo de Digitalización y Anonimizacion
`, fechaFormateada, folio)

	// Crear un nuevo mensaje de correo
	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_EMAIL")) // Email remitente desde el entorno
	m.SetHeader("To", correo)                    // Email destinatario
	m.SetHeader("Subject", "Actualización de donación")
	m.SetBody("text/html", mensaje)

	// Leer la configuración del servidor de correo desde las variables de entorno
	smtpServer := os.Getenv("SMTP_SERVER")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpEmail := os.Getenv("SMTP_EMAIL")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	// Convertir el puerto SMTP de string a int
	port, err := strconv.Atoi(smtpPort)
	if err != nil {
		log.Printf("Error al convertir el puerto SMTP: %v", err) // Log de error
		return fmt.Errorf("error al convertir el puerto SMTP: %v", err)
	}

	// Configuración del servidor de correo
	d := gomail.NewDialer(smtpServer, port, smtpEmail, smtpPassword)

	// Enviar el correo
	log.Println("Intentando enviar el correo...") // Log del intento de envío
	if err := d.DialAndSend(m); err != nil {
		log.Printf("Error al enviar el correo: %v", err) // Log de error
		return fmt.Errorf("error al enviar el correo: %v", err)
	}
	log.Println("Correo enviado con éxito")
	return nil
}

// Función para obtener estudios desde la base de datos con filtros opcionales
func GetEstudios(filtros map[string]interface{}, db *sql.DB) ([]Estudio, error) {
	log.Println("Iniciando consulta de estudios con los filtros:", filtros)

	query := `
	SELECT 
		e.folio, e.fecha_recepcion, e.fecha_devolucion, e.correo, e.curp, e.carrera, 
		e.cuatrimestre, e.area, 
		d.tipo_estudio, d.cantidad_imagenes, d.es_donacion, d.observaciones, d.id 
	FROM estudios e
	LEFT JOIN detalles_estudios d ON e.folio = d.folio
	`

	var args []interface{}
	conditions := []string{}

	// Agregar filtros dinámicamente usando la longitud del slice args para indexarlos correctamente
	if folio, ok := filtros["folio"].(string); ok && folio != "" {
		log.Printf("Añadiendo filtro para 'folio': %s", folio)
		conditions = append(conditions, fmt.Sprintf("e.folio = $%d", len(args)+1))
		args = append(args, folio)
	}
	if correo, ok := filtros["correo"].(string); ok && correo != "" {
		log.Printf("Añadiendo filtro para 'correo': %s", correo)
		conditions = append(conditions, fmt.Sprintf("e.correo = $%d", len(args)+1))
		args = append(args, correo)
	}
	if curp, ok := filtros["curp"].(string); ok && curp != "" {
		log.Printf("Añadiendo filtro para 'curp': %s", curp)
		conditions = append(conditions, fmt.Sprintf("e.curp = $%d", len(args)+1))
		args = append(args, curp)
	}
	if fechaRecepcion, ok := filtros["FechaRecepcion"].(string); ok && fechaRecepcion != "" {
		log.Printf("Añadiendo filtro para 'fecha_recepcion': %s", fechaRecepcion)
		conditions = append(conditions, fmt.Sprintf("DATE(e.fecha_recepcion) = $%d", len(args)+1))
		args = append(args, fechaRecepcion)
	}

	// Solo agregar WHERE si hay condiciones
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
		log.Printf("Consulta SQL con condiciones: %s", query)
	} else {
		log.Println("No se añadieron filtros. La consulta no tiene cláusula WHERE.")
	}

	// Log de la consulta final antes de ejecutar
	log.Printf("Consulta SQL final: %s", query)

	// Ejecutar la consulta
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error al consultar estudios en la base de datos: %v", err)
		return nil, err
	}
	defer rows.Close()

	// Mapa para agrupar estudios por folio
	estudiosMap := make(map[string]*Estudio)
	log.Println("Comenzando a procesar los resultados de la consulta.")

	// Iterar sobre las filas de la consulta
	for rows.Next() {
		var (
			folio            string
			fechaRecepcion   time.Time
			fechaDevolucion  *time.Time
			correo           string
			curp             string
			carrera          string
			cuatrimestre     string
			area             string
			tipoEstudio      string
			cantidadImagenes int
			esDonacion       bool
			observaciones    string
			id               int
		)

		err := rows.Scan(&folio, &fechaRecepcion, &fechaDevolucion, &correo, &curp, &carrera, &cuatrimestre, &area, &tipoEstudio, &cantidadImagenes, &esDonacion, &observaciones, &id)
		if err != nil {
			log.Printf("Error al escanear el estudio con folio %s: %v", folio, err)
			return nil, err
		}

		log.Printf("Procesando estudio con folio: %s", folio)

		detalle := DetalleEstudio{
			TipoEstudio:      tipoEstudio,
			CantidadImagenes: cantidadImagenes,
			EsDonacion:       esDonacion,
			Observaciones:    observaciones,
			Id:               id,
		}

		if estudio, exists := estudiosMap[folio]; exists {
			log.Printf("Estudio con folio %s ya existe, agregando detalle", folio)
			estudio.DetallesEstudios = append(estudio.DetallesEstudios, detalle)
		} else {
			log.Printf("Estudio con folio %s no existe, creando nuevo estudio", folio)
			estudio := &Estudio{
				Folio:            folio,
				FechaRecepcion:   fechaRecepcion,
				FechaDevolucion:  fechaDevolucion,
				Correo:           correo,
				CURP:             curp,
				Carrera:          carrera,
				Cuatrimestre:     cuatrimestre,
				Area:             area,
				DetallesEstudios: []DetalleEstudio{detalle},
			}
			estudiosMap[folio] = estudio
		}
	}

	// Convertir el mapa de estudios en un slice
	var estudios []Estudio
	for _, estudio := range estudiosMap {
		estudios = append(estudios, *estudio)
	}

	// Verificar si hubo algún error durante la iteración
	if err = rows.Err(); err != nil {
		log.Printf("Error en las filas de consulta de estudios: %v", err)
		return nil, err
	}

	// Log de los estudios procesados
	log.Printf("Se han procesado %d estudios.", len(estudios))
	log.Printf("Respuesta enviada al frontend: %v", estudios)

	return estudios, nil
}

// Handler para crear un nuevo estudio y enviar el PDF por correo
func CreateEstudioHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {

	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		log.Printf("Método no permitido: %s", r.Method)
		return
	}

	var estudio Estudio
	err := json.NewDecoder(r.Body).Decode(&estudio)
	if err != nil {
		http.Error(w, "Error al decodificar la solicitud", http.StatusBadRequest)
		log.Printf("Error al decodificar la solicitud: %v", err)
		return
	}

	log.Printf("Creando estudio: %+v", estudio)
	err = CreateEstudio(estudio, db)
	if err != nil {
		http.Error(w, "Error al guardar el estudio", http.StatusInternalServerError)
		log.Printf("Error al guardar el estudio: %v", err)
		return
	}

	// Generar el PDF con todos los detalles de los estudios
	pdfBuffer, err := GeneraPDF(estudio)
	if err != nil {
		http.Error(w, "Error al generar el PDF", http.StatusInternalServerError)
		log.Printf("Error al generar el PDF: %v", err)
		return
	}

	// Enviar el correo con el PDF adjunto
	err = EnviaCorreoPDF(estudio, pdfBuffer)
	if err != nil {
		http.Error(w, "Error al enviar el correo", http.StatusInternalServerError)
		log.Printf("Error al enviar el correo: %v", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Estudio guardado y correo enviado correctamente"))
	log.Println("Estudio guardado y correo enviado correctamente")
}

// Handler para obtener estudios con filtros opcionales
func GetEstudiosHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		log.Printf("Método no permitido: %s", r.Method)
		return
	}

	// Extraer los filtros de los parámetros de consulta
	filtros := map[string]interface{}{
		"folio":          r.URL.Query().Get("folio"),
		"correo":         r.URL.Query().Get("correo"),
		"curp":           r.URL.Query().Get("curp"),
		"FechaRecepcion": r.URL.Query().Get("FechaRecepcion"),
	}

	estudios, err := GetEstudios(filtros, db)
	if err != nil {
		http.Error(w, "Error al obtener los estudios", http.StatusInternalServerError)
		log.Printf("Error al obtener los estudios: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(estudios)
	if err != nil {
		http.Error(w, "Error al codificar los estudios a JSON", http.StatusInternalServerError)
		log.Printf("Error al codificar los estudios a JSON: %v", err)
	}
}

func ConfirmarDigitalizacionHandler(w http.ResponseWriter, r *http.Request) {
	// Decodificar el cuerpo de la solicitud para obtener el correo
	var requestData RequestData
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		log.Printf("Error al decodificar los datos: %v", err) // Log de error
		http.Error(w, "Error al leer el cuerpo de la solicitud", http.StatusBadRequest)
		return
	}
	log.Printf("Solicitud recibida: Correo=%s, Fecha=%s, Folio=%s", requestData.Correo, requestData.Fecha, requestData.Folio) // Log de éxito

	// Llamar a la función que envía el correo
	err = EnviaCorreoConfirmacion(requestData.Correo, requestData.Fecha, requestData.Folio)
	if err != nil {
		log.Printf("Error al enviar el correo: %v", err) // Log de error
		http.Error(w, fmt.Sprintf("Error al enviar el correo: %v", err), http.StatusInternalServerError)
		return
	}

	// Responder con un mensaje de éxito
	log.Println("Correo enviado correctamente") // Log de éxito
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Correo enviado correctamente",
	})
}
