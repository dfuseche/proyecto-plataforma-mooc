<#
.SYNOPSIS
  Corre el Escenario 1 (actividad academica concurrente) en varios niveles
  de carga separados, como exige la Entrega 2: una linea base con carga
  baja, al menos tres niveles crecientes, y una repeticion cerca del
  limite para comprobar estabilidad.

.DESCRIPTION
  Cada nivel es una corrida independiente y corta de load-test.js
  (LEVEL_RUN=true, ~4-5 min en vez de los ~23 min del perfil completo de
  rampa a 2000 VUs), con su propio --summary-export a un JSON separado en
  loadtests/results/escenario1/. Corre esto DESDE TU MAQUINA (fuera de las
  dos VMs de la aplicacion), tal como exige el enunciado.

  Antes de correrlo:
    - Verifica que el seed este cargado (incluye el quiz) contra la BD del
      despliegue actual.
    - Arranca en paralelo (otra terminal / el monitor.ps1) la captura de
      metricas de infraestructura, para poder cruzarlas con estos niveles.
    - Ajusta los niveles de $Niveles segun lo que veas: si el nivel 3 ya
      esta claramente degradado (thresholds en rojo), no hace falta subir
      mas — repite ese mismo nivel para confirmar que el punto de
      degradacion es estable, y documenta que no corresponde a la
      capacidad maxima si el presupuesto no permite llegar a saturacion.

.EXAMPLE
  ./run_escenario1_niveles.ps1 -BaseUrl http://35.209.105.199
#>

param(
  [Parameter(Mandatory = $true)][string]$BaseUrl,
  [int[]]$Niveles = @(10, 50, 150, 400),   # linea base + 3 niveles crecientes
  [switch]$RepetirUltimoNivel = $true,      # repite el ultimo nivel para comprobar estabilidad cerca del limite
  [int]$PausaEntreCorridasSegundos = 30
)

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $scriptDir)
$resultsDir = Join-Path $repoRoot "loadtests/results/escenario1"
New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$etiquetas = @("00_linea_base")
for ($i = 1; $i -lt $Niveles.Count; $i++) { $etiquetas += "0$i`_nivel$i" }
if ($RepetirUltimoNivel) {
  $Niveles += $Niveles[-1]
  $etiquetas += "0$($Niveles.Count - 1)`_repeticion_cerca_del_limite"
}

Write-Host "=== Escenario 1: $($Niveles.Count) corridas contra $BaseUrl ===" -ForegroundColor Cyan
Write-Host "Niveles (MAX_VUS): $($Niveles -join ', ')" -ForegroundColor Cyan

for ($idx = 0; $idx -lt $Niveles.Count; $idx++) {
  $vus = $Niveles[$idx]
  $etiqueta = $etiquetas[$idx]
  # No tiene sentido loguear 300 usuarios del pool para probar con 10 VUs:
  # se escala el pool con el nivel, con un piso de 20 para no agotar tokens
  # distintos en checks de inscripcion/quiz.
  $poolSize = [Math]::Max(20, [Math]::Min(300, $vus * 2))

  $jsonOut = Join-Path $resultsDir "$timestamp`_$etiqueta`_vus$vus.json"
  $logOut = Join-Path $resultsDir "$timestamp`_$etiqueta`_vus$vus.log"

  Write-Host ""
  Write-Host "--- [$($idx + 1)/$($Niveles.Count)] $etiqueta : MAX_VUS=$vus POOL_SIZE=$poolSize ---" -ForegroundColor Yellow

  k6 run `
    -e BASE_URL=$BaseUrl `
    -e MAX_VUS=$vus `
    -e POOL_SIZE=$poolSize `
    -e LEVEL_RUN=true `
    --summary-export=$jsonOut `
    (Join-Path $scriptDir "load-test.js") 2>&1 | Tee-Object -FilePath $logOut

  Write-Host "Resultado guardado en: $jsonOut" -ForegroundColor Green

  if ($idx -lt $Niveles.Count - 1) {
    Write-Host "Pausa de $PausaEntreCorridasSegundos s antes del siguiente nivel (deja drenar conexiones)..." -ForegroundColor DarkGray
    Start-Sleep -Seconds $PausaEntreCorridasSegundos
  }
}

Write-Host ""
Write-Host "=== Listo. Resultados en $resultsDir ===" -ForegroundColor Cyan
