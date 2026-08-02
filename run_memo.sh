#!/bin/bash
cd /home/bugra/Documents/memo && go run -tags "sqlite_fts5" . --headless --port 8090 &
cd /home/bugra/Documents/memo/frontend && flutter run -d linux
