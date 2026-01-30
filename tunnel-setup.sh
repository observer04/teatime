#!/bin/bash
# TeaTime - Quick Tunnel Setup for Mobile Testing
# Choose your tunnel provider

echo "🚇 TeaTime Tunnel Setup"
echo "======================="
echo ""
echo "Choose a tunnel provider:"
echo "1) Cloudflare Tunnel (cloudflared) - Recommended, free, no signup"
echo "2) ngrok - Popular alternative, requires free account"
echo "3) Check if services are running first"
echo ""
read -p "Enter choice [1-3]: " choice

case $choice in
  1)
    echo ""
    echo "Installing cloudflared..."
    
    # Check if already installed
    if command -v cloudflared &> /dev/null; then
      echo "✓ cloudflared already installed"
    else
      # Install cloudflared
      wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
      sudo dpkg -i cloudflared-linux-amd64.deb
      rm cloudflared-linux-amd64.deb
      echo "✓ cloudflared installed"
    fi
    
    echo ""
    echo "Starting tunnels..."
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    # Start backend tunnel in background
    cloudflared tunnel --url http://localhost:8080 > /tmp/backend-tunnel.log 2>&1 &
    BACKEND_PID=$!
    sleep 3
    
    # Start frontend tunnel in background  
    cloudflared tunnel --url http://localhost:5173 > /tmp/frontend-tunnel.log 2>&1 &
    FRONTEND_PID=$!
    sleep 3
    
    # Extract URLs
    BACKEND_URL=$(grep -oP 'https://[a-z0-9-]+\.trycloudflare\.com' /tmp/backend-tunnel.log | head -1)
    FRONTEND_URL=$(grep -oP 'https://[a-z0-9-]+\.trycloudflare\.com' /tmp/frontend-tunnel.log | head -1)
    
    echo "✓ Tunnels started!"
    echo ""
    echo "📱 Access from your phone:"
    echo "   Frontend: $FRONTEND_URL"
    echo "   Backend:  $BACKEND_URL"
    echo ""
    echo "⚠️  IMPORTANT: Update frontend to use the backend tunnel URL"
    echo "   The app will try to call localhost:8080 which won't work through tunnel"
    echo ""
    echo "💡 To stop tunnels:"
    echo "   kill $BACKEND_PID $FRONTEND_PID"
    echo ""
    echo "Press Ctrl+C to stop..."
    
    # Keep script running
    wait
    ;;
    
  2)
    echo ""
    echo "Installing ngrok..."
    
    if command -v ngrok &> /dev/null; then
      echo "✓ ngrok already installed"
    else
      # Install ngrok
      curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
      echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
      sudo apt update && sudo apt install -y ngrok
      echo "✓ ngrok installed"
    fi
    
    echo ""
    echo "⚠️  ngrok requires authentication"
    echo "   1. Sign up free at: https://dashboard.ngrok.com/signup"
    echo "   2. Get your authtoken from: https://dashboard.ngrok.com/get-started/your-authtoken"
    echo "   3. Run: ngrok config add-authtoken YOUR_TOKEN"
    echo ""
    read -p "Have you added your authtoken? (y/n): " confirmed
    
    if [[ $confirmed == "y" ]]; then
      echo ""
      echo "Starting tunnels..."
      echo "Open these URLs in separate terminals:"
      echo ""
      echo "Terminal 1: ngrok http 8080 --domain=<your-domain>.ngrok-free.app"
      echo "Terminal 2: ngrok http 5173"
      echo ""
    fi
    ;;
    
  3)
    echo ""
    echo "Checking services..."
    echo ""
    
    # Check if backend is running
    if curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
      echo "✓ Backend is running on port 8080"
    else
      echo "✗ Backend is NOT running"
      echo "  Start with: cd /home/observer/projects/teatime && docker compose up -d"
    fi
    
    # Check if frontend is running
    if curl -s http://localhost:5173 > /dev/null 2>&1; then
      echo "✓ Frontend is running on port 5173"
    else
      echo "✗ Frontend is NOT running"
      echo "  Start with: cd /home/observer/projects/teatime && docker compose up -d"
    fi
    
    echo ""
    ;;
    
  *)
    echo "Invalid choice"
    exit 1
    ;;
esac
