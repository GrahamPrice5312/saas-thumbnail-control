#!/bin/sh
set -eu

curl -sS -X POST http://localhost:8080/admin/tenants \
  -H 'Content-Type: application/json' \
  -d '{"id":"acme"}'
curl -sS -X POST http://localhost:8080/admin/tenants/acme/state \
  -H 'Content-Type: application/json' \
  -d '{"state":"active"}'
printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' \
  | openssl base64 -d -A \
  | curl -sS -X POST http://localhost:8080/tenants/acme/thumbnails \
  -H 'Idempotency-Key: acme-product-42' \
  -F 'image=@-;filename=product.png'
