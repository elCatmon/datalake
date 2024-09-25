import sys
import pydicom
import os
import logging

# Configuración del logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

def dicom_anon(input_file_path, output_file_path):
    logging.info(f"Iniciando anonimización para el archivo: {input_file_path}")

    # Verificar si el archivo existe
    if not os.path.exists(input_file_path):
        logging.error(f"El archivo {input_file_path} no existe.")
        sys.exit(1)

    # Cargar el archivo DICOM
    try:
        dataset = pydicom.dcmread(input_file_path)
        logging.info("Archivo DICOM cargado correctamente.")
    except Exception as e:
        logging.error(f"Error al cargar el archivo DICOM: {e}")
        sys.exit(1)

    # Lista de atributos a eliminar o anonimizar
    fields_to_remove = [
        'PatientName', 'PatientID', 'PatientAddress',
        'PatientTelephoneNumbers', 'PatientInsurancePlanCodeSequence'
    ]

    # Eliminar los datos personales
    for field in fields_to_remove:
        if field in dataset:
            logging.info(f"Eliminando el campo: {field}")
            delattr(dataset, field)

    # Guardar el archivo DICOM anonimizado en la ruta especificada
    try:
        dataset.save_as(output_file_path)
        logging.info(f"Archivo DICOM anonimizado guardado en: {output_file_path}")
    except Exception as e:
        logging.error(f"Error al guardar el archivo DICOM anonimizado: {e}")
        sys.exit(1)

    return output_file_path

if __name__ == "__main__":
    if len(sys.argv) != 3:
        logging.error("Uso incorrecto. Se requieren dos argumentos: (ruta del archivo DICOM, ruta del archivo de salida).")
        sys.exit(1)

    input_file_path = sys.argv[1]
    output_file_path = sys.argv[2]
    logging.info(f"Script iniciado con el archivo: {input_file_path}")
    dicom_anon(input_file_path, output_file_path)
