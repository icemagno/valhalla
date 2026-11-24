package crc

import "errors"

var (
	ErrFederationExecutionAlreadyExists = errors.New("hla crc: execucao de federacao ja existe")
	ErrFederationExecutionDoesNotExist  = errors.New("hla crc: execucao de federacao nao encontrada")
	ErrFederateNameAlreadyInUse         = errors.New("hla crc: nome de federado ja em uso")
	ErrFederateNotFound                 = errors.New("hla crc: federado nao encontrado na federacao")
	ErrSyncPointLabelNotUnique          = errors.New("hla crc: ponto de sincronizacao ja registrado")
	ErrSyncPointNotFound                = errors.New("hla crc: ponto de sincronizacao nao encontrado")
	ErrObjectInstanceNameNotUnique      = errors.New("hla crc: nome de instancia de objeto nao exclusivo")
	ErrObjectInstanceNotFound           = errors.New("hla crc: instancia de objeto nao encontrada")
	ErrInvalidProtocolPreamble          = errors.New("hla crc: preambulo de protocolo desconhecido")
)
