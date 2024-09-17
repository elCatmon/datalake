import os
import hashlib
import random
import string
from bson import ObjectId
import gridfs
from pymongo import MongoClient
from datetime import datetime
from pydicom import dcmread
from tkinter import Tk, filedialog

# Conexión a MongoDB
client = MongoClient('mongodb://localhost:27017/')
db = client['bdmdm']
fs = gridfs.GridFS(db, collection='imagenes')

# Función para calcular hash de un archivo (nombre del archivo en este caso)
def calculate_file_hash(filename):
    sha256_hash = hashlib.sha256(filename.encode())
    return sha256_hash.hexdigest()

# Función para generar un ID de estudio único de 12 dígitos
def generate_study_id():
    return ''.join(random.choices(string.digits, k=12))

# Función para subir un archivo DICOM y JPG a GridFS
def upload_files_to_gridfs(dicom_path, jpg_path):
    dicom_id = None
    jpg_id = None

    # Subir el archivo DICOM
    with open(dicom_path, 'rb') as dicom_file:
        dicom_id = fs.put(dicom_file, filename=os.path.basename(dicom_path), contentType="application/dicom")

    # Subir el archivo JPG
    if os.path.exists(jpg_path):
        with open(jpg_path, 'rb') as jpg_file:
            jpg_id = fs.put(jpg_file, filename=os.path.basename(jpg_path), contentType="image/jpeg")

    return str(dicom_id), str(jpg_id)

# Función para extraer información DICOM
def extract_dicom_info(dicom_path):
    dicom_data = dcmread(dicom_path)
    
    # Extraer sexo y convertir a M/F
    sexo = dicom_data.PatientSex if 'PatientSex' in dicom_data else "N/A"
    sexo = "M" if sexo == "M" else "F" if sexo == "F" else "N/A"

    # Extraer edad
    edad = dicom_data.PatientAge if 'PatientAge' in dicom_data else None
    edad = int(edad[:-1]) if edad else None

    # Extraer fecha de nacimiento
    fecha_nacimiento = dicom_data.PatientBirthDate if 'PatientBirthDate' in dicom_data else None
    if fecha_nacimiento:
        fecha_nacimiento = datetime.strptime(fecha_nacimiento, '%Y%m%d')
    
    # Extraer fecha de estudio
    fecha_estudio = dicom_data.StudyDate if 'StudyDate' in dicom_data else None
    if fecha_estudio:
        fecha_estudio = datetime.strptime(fecha_estudio, '%Y%m%d')

    # Extraer región
    region = dicom_data.StudyDescription if 'StudyDescription' in dicom_data else "Desconocido"

    return sexo, edad, fecha_nacimiento, fecha_estudio, region

# Función para crear un documento en la colección estudios
def create_estudio_document(estudio_info, image_data):
    estudio_document = {
        "estudio_ID": estudio_info["estudio_ID"],
        "region": estudio_info["region"],
        "hash": calculate_file_hash(estudio_info["filename"]),
        "status": "Aceptado",
        "estudio": estudio_info["estudio"],
        "sexo": estudio_info["sexo"],
        "edad": estudio_info["edad"],
        "fecha_nacimiento": estudio_info["fecha_nacimiento"],
        "fecha_estudio": estudio_info["fecha_estudio"],
        "imagenes": image_data,  # Aquí se almacenan los ObjectId de las imágenes
        "diagnostico": [
            {
                "proyeccion": "N/A",
                "hallazgos": "N/A"
            }
        ]
    }

    # Insertar el documento en la colección 'estudios'
    db.estudios.insert_one(estudio_document)
    print(f"Estudio insertado con ID: {estudio_document['_id']}")

# Función para seleccionar archivos DICOM y procesarlos
def process_files():
    # Abrir un diálogo para seleccionar archivos DICOM
    Tk().withdraw()  # Ocultar la ventana principal de Tkinter
    dicom_paths = filedialog.askopenfilenames(title="Selecciona los archivos DICOM", filetypes=[("DICOM files", "*.dcm")])

    estudio_info = {
        "estudio_ID": generate_study_id(),  # Generar un ID único para el estudio
        "estudio": "TomografiaComputarizada",  # Tipo de estudio por defecto
        "sexo": "N/A",
        "edad": None,
        "fecha_nacimiento": None,
        "fecha_estudio": None,
        "region": "Desconocido",
        "filename": None
    }

    image_data = []

    for dicom_path in dicom_paths:
        file_name = os.path.basename(dicom_path)
        jpg_path = dicom_path.replace(".dcm", ".jpg")

        if os.path.exists(jpg_path):
            # Extraer información del archivo DICOM
            sexo, edad, fecha_nacimiento, fecha_estudio, region = extract_dicom_info(dicom_path)
            
            # Actualizar la información del estudio
            estudio_info["sexo"] = sexo
            estudio_info["edad"] = edad
            estudio_info["fecha_nacimiento"] = fecha_nacimiento
            estudio_info["fecha_estudio"] = fecha_estudio
            estudio_info["region"] = region
            estudio_info["filename"] = file_name

            # Subir los archivos a GridFS
            dicom_id, jpg_id = upload_files_to_gridfs(dicom_path, jpg_path)

            # Agregar los datos de la imagen al documento del estudio
            image_data.append({
                "dicom": dicom_id,
                "imagen": jpg_id,
                "anonimizada": True
            })
        else:
            print(f"No se encontró un archivo JPG asociado para {dicom_path}")

    # Crear documento de estudio
    if image_data:
        create_estudio_document(estudio_info, image_data)

if __name__ == "__main__":
    process_files()
