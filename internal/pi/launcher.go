package pi

import (
	"fmt"
	"os"
	"path/filepath"

	"context"

	"github.com/jparrill/auriga-cli/internal/config"
	"github.com/jparrill/auriga-cli/internal/exec"
	"github.com/jparrill/auriga-cli/internal/ui"
	"github.com/spf13/viper"
)

const systemMDTemplate = `# Proyecto Astro — Fix Session

Modelo original: %s
Backend: %s
Estado: %s
Archivos generados: %d

## Objetivo

Hacer que npm install && npm run build pase sin errores.
Despues, verificar que npm run dev -- --host 0.0.0.0 sirve correctamente en puerto 4321.

## Errores comunes (observados en este benchmark)

- NO usar @astrojs/node — es un sitio estatico, no necesita adapter
- Usar astro@latest (^5.x), no versiones viejas
- NO usar getStaticProps (eso es Next.js, no Astro)
- El campo site en astro.config.mjs debe ser URL valida como 'https://example.github.io/auriga-lab'
- NO poner adapter: null — omitir el campo adapter completamente
- Todas las imports deben tener su dependencia en package.json
- Usar output: 'static' en astro.config.mjs
- NO importar modulos que no estan en package.json

## Workflow

1. npm install --legacy-peer-deps
2. npm run build (verificar que compila)
3. Si falla, analizar el error y corregir
4. Repetir hasta que build pase
5. npm run dev -- --host 0.0.0.0 (verificar que sirve en puerto 4321)

## Reglas

- Edita solo los archivos necesarios para que funcione
- No regeneres todo el proyecto desde cero
- Haz cambios incrementales y verifica despues de cada uno
`

func Bin() string {
	return config.ExpandHome(viper.GetString("pi.bin"))
}

func WriteSystemMD(projectDir, model, backend, status string, filesCreated int) error {
	piDir := filepath.Join(projectDir, ".pi")
	if err := os.MkdirAll(piDir, 0755); err != nil {
		return err
	}

	content := fmt.Sprintf(systemMDTemplate, model, backend, status, filesCreated)
	return os.WriteFile(filepath.Join(piDir, "SYSTEM.md"), []byte(content), 0644)
}

func Launch(ctx context.Context, projectDir, modelID string) error {
	bin := Bin()
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("pi binary not found: %s", bin)
	}

	ui.Info(fmt.Sprintf("Launching Pi with model %s in %s", modelID, projectDir))
	fmt.Printf("\n  %s\n", ui.BoldStyle.Render("═══════════════════════════════════════════════════"))
	fmt.Printf("  %s\n", ui.BoldStyle.Render("Pi session — Ctrl+C to exit"))
	fmt.Printf("  %s\n\n", ui.BoldStyle.Render("═══════════════════════════════════════════════════"))

	return exec.RunStreaming(ctx, bin, []string{"--model", modelID}, exec.RunOpts{Dir: projectDir})
}
