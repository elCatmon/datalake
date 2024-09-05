import sys
import pydicom
import os

def dicom_anon(file_path):
    # Cargar el archivo DICOM
    dataset = pydicom.dcmread(file_path)
    # Establecer ruta de archivo anonimizado en la misma ubicación
    dicom_file_path = os.path.splitext(file_path)[0] + "_M.dcm"
    # Lista de atributos a eliminar o anonimizar
    fields_to_remove = [
        'PatientName', 'PatientID', 'PatientAge', 'PatientAddress',
        'PatientTelephoneNumbers', 'PatientInsurancePlanCodeSequence'
    ]
    # Eliminar los datos personales
    for field in fields_to_remove:
        if field in dataset:
            delattr(dataset, field)
    # Guardar el archivo DICOM anonimizado en la misma ubicación
    dataset.save_as(dicom_file_path)
    return dicom_file_path

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Uso: python script.py <ruta_del_archivo_dicom>")
        sys.exit(1)
    file_path = sys.argv[1]
    dicom_anon(file_path)
