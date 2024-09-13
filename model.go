package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User representa un usuario en la base de datos.
type User struct {
	ID           string `json:"usuario_id"`
	Nombre       string `json:"nombre"`
	ApellidoP    string `json:"apellido_Paterno"`
	ApellidoM    string `json:"apellido_Materno"`
	Curp         string `json:"curp"`
	Sexo         int    `json:"sexo"`
	Estado       string `json:"estado"`
	Ciudad       string `json:"ciudad"`
	Municipio    string `json:"municipio"`
	Correo       string `json:"correo"`
	Contrasena   string `json:"contrasena"`
	Tipo_Usuario int    `json:"tipo_Usuario"`
	Proyecto     string `json:"proyecto"`
}

// acciones blockchain
type Acciones struct {
	NoOperacion     string    `json:"NoOperacion"`
	Quien_Ejec      string    `json:"Quien_ejec"`
	Usuario_Enc     string    `json:"Usuario_Enc"`
	Accion          string    `json:"Accion"`
	DatoClinicoO_ID string    `json:"ID_datoClinicoO"`
	DatoClinicoM_ID string    `json:"ID_datoClinicoM"`
	Fecha           time.Time `json:"fecha"`
}

type Proyectos struct {
	ProyectoID     string `json:"proyecto_ID"`
	UsuarioID      string `json:"usuario_ID"`
	NombreProyecto string `json:"nombre_Proyecto"`
	Financiado     string `json:"financiado"`
	NombreFin      string `json:"nombre_Fin"`
	InicioProy     string `json:"inicio_Proyecto"`
	FinProy        string `json:"fin_Proyecto"`
	Responsable    string `json:"responsable"`
	Empresa        string `json:"empresa"`
}

type Donaciones struct {
	DonacionID  string    `json:"donacion_ID"`
	UsuarioID   string    `json:"usuario_ID"`
	Fecha       time.Time `json:"fecha"`
	Transaccion string    `json:"transaccion"`
}

type Consultas struct {
	ConsultaID string    `json:"consulta_ID"`
	UsuarioID  string    `json:"usuario_ID"`
	Fecha      time.Time `json:"fecha"`
	Consulta   string    `json:"consulta"`
}

// FileDocument representa un documento en la colección `imagenes.files`.
type FileDocument struct {
	ID          primitive.ObjectID `bson:"_id"`
	Filename    string             `bson:"filename"`
	EstudioType string             `bson:"estudio"`
}

type Imagen struct {
	Dicom       string `bson:"dicom"`
	Imagen      string `bson:"imagen"`
	Anonimizada bool   `bson:"anonimizada"`
}

type Diagnostico struct {
	Proyeccion string `bson:"proyeccion"`
	Hallazgos  string `bson:"hallazgos"`
}

type EstudioDocument struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	EstudioID       string             `bson:"estudio_ID"`
	Region          string             `bson:"region"`
	Hash            string             `bson:"hash"`
	Status          string             `bson:"status"`
	Estudio         string             `bson:"estudio"`
	Sexo            string             `bson:"sexo"`
	Edad            int                `bson:"edad"`
	FechaNacimiento time.Time          `bson:"fecha_nacimiento"`
	FechaEstudio    time.Time          `bson:"fecha_estudio"`
	Imagenes        []Imagen           `bson:"imagenes"`
	Diagnostico     []Diagnostico      `bson:"diagnostico"`
}
