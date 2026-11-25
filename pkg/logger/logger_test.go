package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestLogger_BasicLevelsAndFlush(t *testing.T) {
	var buf bytes.Buffer
	log := &valhallaLogger{
		level:  LevelDebug,
		output: &buf,
		module: "TEST_MOD",
	}

	log.Debug("mensagem de debug: %d", 42)
	log.Info("mensagem de info")
	log.Warn("alerta de warn")
	log.Error("erro ocorrido")
	log.Critical("falha critica")
	log.Trace("trace ignorado porque level e debug")

	Flush()

	output := buf.String()
	if !strings.Contains(output, "mensagem de debug: 42") {
		t.Errorf("esperava conter debug: %s", output)
	}
	if !strings.Contains(output, "mensagem de info") {
		t.Errorf("esperava conter info: %s", output)
	}
	if !strings.Contains(output, "alerta de warn") {
		t.Errorf("esperava conter warn: %s", output)
	}
	if !strings.Contains(output, "erro ocorrido") {
		t.Errorf("esperava conter error: %s", output)
	}
	if !strings.Contains(output, "falha critica") {
		t.Errorf("esperava conter critical: %s", output)
	}
	if strings.Contains(output, "trace ignorado") {
		t.Errorf("nao esperava conter trace: %s", output)
	}
}

func TestLogger_ConcurrentLogging(t *testing.T) {
	var buf bytes.Buffer
	log := &valhallaLogger{
		level:  LevelInfo,
		output: &buf,
		module: "CONC_MOD",
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				log.Info("thread %d msg %d", id, j)
			}
		}(i)
	}
	wg.Wait()
	Flush()
}
