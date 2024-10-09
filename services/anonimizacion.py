import sys
import pydicom
import os

def dicom_anon(input_file_path, output_file_path):
    # Verificar si el archivo existe
    if not os.path.exists(input_file_path):
        sys.exit(1)

    # Cargar el archivo DICOM
    try:
        dataset = pydicom.dcmread(input_file_path)
    except Exception as e:
        sys.exit(1)

    # Lista de atributos a eliminar o anonimizar
    fields_to_remove = [
        'PatientName', 'PatientID', 'PatientAddress',
        'PatientTelephoneNumbers', 'PatientInsurancePlanCodeSequence'
    ]

    # Eliminar los datos personales
    for field in fields_to_remove:
        if field in dataset:
            delattr(dataset, field)

    # Guardar el archivo DICOM anonimizado en la ruta especificada
    try:
        dataset.save_as(output_file_path)
    except Exception as e:
        sys.exit(1)

    return output_file_path

if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(1)

    input_file_path = sys.argv[1]
    output_file_path = sys.argv[2]
    dicom_anon(input_file_path, output_file_path)
