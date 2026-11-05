package main

import (
	"sync"
	"time"
)

type cotacaoSalva struct {
	saida  SaidaCotacao
	valida time.Time
}

type envioSalvo struct {
	impressao string
	saida     SaidaContratacao
}

// armazem guarda cotações e envios em memória; o relógio é injetável para
// testar expiração.
type armazem struct {
	mu       sync.Mutex
	agora    func() time.Time
	seq      int
	cotacoes map[string]cotacaoSalva
	envios   map[string]envioSalvo // por chave de idempotência
	usadas   map[string]string     // cotacao_id -> chave que a consumiu
}

func novoArmazem() *armazem {
	return &armazem{
		agora:    time.Now,
		cotacoes: map[string]cotacaoSalva{},
		envios:   map[string]envioSalvo{},
		usadas:   map[string]string{},
	}
}
