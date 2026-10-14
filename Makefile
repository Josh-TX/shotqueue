.NOTPARALLEL:
.PHONY: build frontend backend clean

build: frontend backend

frontend:
	cd frontend && npm ci && npm run build

backend:
	cd backend && go build -trimpath -tags embed_frontend -ldflags="-s -w -X main.version=dev" -o ../shotqueue .

clean:
	rm -rf backend/internal/webui/dist shotqueue
