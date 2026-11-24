package lrc

import "errors"

var (
	ErrNotConnected                = errors.New("hla lrc: federado nao conectado ao rti")
	ErrAlreadyConnected            = errors.New("hla lrc: federado ja conectado ao rti")
	ErrFederateNotExecutionMember  = errors.New("hla lrc: federado nao ingressou em nenhuma federacao")
	ErrFederateAlreadyExecutionMember = errors.New("hla lrc: federado ja eh membro de uma federacao")
	ErrConnectionClosed            = errors.New("hla lrc: conexao de transporte encerrada")
	ErrRequestTimeout              = errors.New("hla lrc: tempo limite esgotado aguardando resposta do crc")
	ErrInvalidResponse             = errors.New("hla lrc: resposta invalida do crc")
)
