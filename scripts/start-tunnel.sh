#!/bin/bash
docker run -d \
  --name cloudflared-tunnel \
  --restart unless-stopped \
  --network host \
  cloudflare/cloudflared:latest \
  tunnel --no-autoupdate run --token eyJhIjoiYjQ3MGU2MjllMjkyNjgyNTBjMzQ4MzkxYzkzZDUyMWYiLCJ0IjoiYmFiNTdhYzgtOWQzNy00M2ViLTk3MTQtN2I1MTY3ZWRjNzdiIiwicyI6Ik9XWmpNMkpsWXpNdE9HWTNOaTAwWTJGbExUZ3haR1l0WXprMk16QTNNakpsTnpNMCJ9
