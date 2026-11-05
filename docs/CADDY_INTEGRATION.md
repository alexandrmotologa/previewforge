# Caddy Server & Wildcard TLS Integration

PreviewForge integrates with Caddy to provide automatic wildcard SSL certificates and HTTPS routing for preview subdomains.

## 1. Wildcard DNS Setup

To route all pull request previews to your VPS, add a wildcard DNS record in your DNS provider (Cloudflare, Namecheap, AWS Route 53):

```
Type: A
Name: *.preview
Value: <YOUR_VPS_PUBLIC_IP>
TTL: Auto / 300
```

This points all incoming requests matching `pr-*.preview.yourdomain.com` to your VPS.

## 2. Setting Up Caddy with Cloudflare DNS Challenge

Wildcard certificates (`*.preview.yourdomain.com`) require the ACME DNS-01 challenge. If you use Cloudflare:

1. Install Caddy with the Cloudflare DNS module:
   ```bash
   caddy add-package github.com/caddy-dns/cloudflare
   ```

2. Configure `/etc/caddy/Caddyfile`:
   ```caddyfile
   {
       admin 127.0.0.1:2019
   }

   # Wildcard preview router
   *.preview.yourdomain.com {
       tls {
           dns cloudflare {env.CLOUDFLARE_API_TOKEN}
       }

       # Proxy to PreviewForge daemon
       reverse_proxy 127.0.0.1:9850
   }

   # PreviewForge Web Studio Dashboard
   preview.yourdomain.com {
       tls {
           dns cloudflare {env.CLOUDFLARE_API_TOKEN}
       }
       reverse_proxy 127.0.0.1:9850
   }
   ```

3. Start Caddy:
   ```bash
   sudo systemctl restart caddy
   ```

## 3. Running PreviewForge in Caddy Mode

Start PreviewForge pointing to the base domain and Caddy Admin API:

```bash
previewforge serve \
  --domain preview.yourdomain.com \
  --port 9850 \
  --proxy-mode caddy \
  --caddy-url http://127.0.0.1:2019
```

When a pull request opens, PreviewForge assigns port `31002`, builds the stack, and registers the route `pr-42.preview.yourdomain.com -> 127.0.0.1:31042` with Caddy via HTTP API.
