package services

import "log"

func ObtenerDiagnosticoIA(imagen string) (string, string, string, error) {
	log.Println(imagen)
	hallazgos := "Hallazgos dommkdmdf "
	impresion := "Impresion imp"
	observaciones := "Observaciones observadas"

	return hallazgos, impresion, observaciones, nil
}
