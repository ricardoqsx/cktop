# Publicar dtop

El workflow `.github/workflows/release-dtop.yml` publica únicamente `dtop` cuando recibe un tag `dtop-vX.Y.Z`.

## Preparar Minisign una sola vez

Generar una clave dedicada sin contraseña para uso exclusivo del secreto cifrado de GitHub Actions:

```bash
minisign -G -W -p configs/dtop.minisign.pub -s dtop.minisign.key
base64 < dtop.minisign.key | tr -d '\n' | gh secret set DTOP_MINISIGN_SECRET_KEY_B64
```

La clave privada no debe añadirse al repositorio, logs, issues ni release assets. Debe mantenerse una copia de respaldo segura fuera de GitHub. La clave pública `configs/dtop.minisign.pub` sí se versiona como trust anchor; el workflow exige que coincida con el secreto, la copia en los assets y la incrusta en el instalador.

En GitHub debe configurarse además un ruleset que impida mover o eliminar tags `dtop-v*` y, si la opción está disponible para el repositorio, habilitar releases inmutables.

## Publicar

1. Confirmar que CI pasó sobre el commit que se etiquetará.
2. Actualizar las notas versionadas cuando corresponda.
3. Crear y subir el tag sin moverlo posteriormente:

```bash
git tag -a dtop-v0.4.0 -m "dtop v0.4.0"
git push origin dtop-v0.4.0
```

El workflow valida, genera cuatro archivos deterministas, firma checksums, renderiza y prueba el instalador, crea attestations y publica el release. Rechaza sobrescribir un release existente.

## Validar después de publicar

Ejecutar el comando versionado documentado en `README.md` sobre Linux o macOS, comprobar `dtop --version` y confirmar que una segunda ejecución conserva `dtop.conf`.
