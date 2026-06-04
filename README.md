# Rinha de Backend 2026 - Go

Implementacao em Go para a Rinha de Backend 2026.

A solucao usa duas instancias da API Go atras de um Nginx. O endpoint combina um score heuristico/logistico rapido com uma busca vetorial aproximada em um indice IVF gerado no build da imagem a partir do dataset oficial de referencias.

## Stack

- Go 1.22
- Fasthttp
- Parser JSON manual no hot path
- Indice vetorial quantizado em `int16`
- Nginx 1.27 Alpine
- Docker Compose

## Estrutura

```text
cmd/api/main.go                   Bootstrap da aplicacao
cmd/build_index/main.go           Geracao do indice vetorial no build
internal/handler/fraud_handler.go Handlers HTTP
internal/service/fraud_scorer.go  Motor de decisao antifraude
internal/service/body_parser.go   Parser manual da requisicao
internal/service/vector_index.go  Leitura e consulta do indice IVF
internal/service/date_math.go     Calculos rapidos de data
docker-compose.yml                2 APIs + 1 Nginx
nginx.conf                        Load balancer na porta 9999
Dockerfile                        Build multi-stage da imagem Go
```

## Estrategia

Durante o build, a imagem baixa `references.json.gz`, quantiza os vetores oficiais para `int16`, particiona os dados por sinais binarios fortes e gera um indice IVF com 512 clusters por particao.

No runtime, a API faz o parse manual do payload, calcula primeiro um score heuristico rapido e chama a busca vetorial apenas em uma faixa de decisao mais sensivel. Isso reduz falsos positivos sem pagar o custo da busca vetorial em todas as requisicoes.

## Como Rodar

```bash
docker compose up -d --build
```

A API fica disponivel em:

```text
http://localhost:9999
```

## Endpoints

- `GET /ready`
- `POST /fraud-score`

Resposta esperada:

```json
{
  "approved": true,
  "fraud_score": 0
}
```

## Teste Local

Na pasta dos testes:

```bash
k6 run smoke.js
k6 run test.js
```

Ultimo resultado local:

```text
p99: 1.09ms
http_errors: 0
false_positive_detections: 5
false_negative_detections: 6
weighted_errors_E: 23
final_score: 5547.00
```
