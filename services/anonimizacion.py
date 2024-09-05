#importacion de librerias
import pydicom
import os
from pydicom.pixel_data_handlers.util import apply_voi_lut

class anon():
    #Funcion para anonimizar los archivos
    def dicom_anon(file_path):
        # Cargar el archivo DICOM
        dataset = pydicom.dcmread(file_path)
        # Establecer ruta de archivo anonimizado
        dicom_file_path = os.path.splitext(file_path)[0] + "_M.dcm"
        # Lista de atributos a eliminar o anonimizar
        fields_to_remove = [
            'PatientName', 'PatientID', 'PatientAge', 'PatientAddress', 'PatientTelephoneNumbers', 'PatientInsurancePlanCodeSequence'
        ]
        # Eliminar los datos personales
        for field in fields_to_remove:
            if field in dataset:
                delattr(dataset, field)
        # Guardar el archivo DICOM anonimizado
        dataset.save_as(dicom_file_path)
        return dicom_file_path