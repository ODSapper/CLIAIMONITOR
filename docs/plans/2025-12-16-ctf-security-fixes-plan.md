# CTF Security Fixes Plan

**Date:** 2025-12-16
**Status:** Planning
**Estimated Time:** 4-6 hours total, ~2 hours with parallel execution

---

## Executive Summary

This plan addresses all security vulnerabilities found during CTF prep testing. Fixes are organized into parallel workstreams to minimize total remediation time.

---

## Vulnerability Summary

| ID | Severity | Component | Issue |
|----|----------|-----------|-------|
| MSS-SEC-001 | CRITICAL | MSS | /api/status endpoint missing authentication |
| CRIT-01 | CRITICAL | WordPress | XML-RPC enabled with multicall amplification |
| CRIT-02 | CRITICAL | WordPress | Username enumeration via login errors |
| CRIT-03 | CRITICAL | WordPress | REST API user enumeration |
| CRIT-04 | CRITICAL | WordPress | No login rate limiting |
| CRIT-05 | CRITICAL | WordPress | Version disclosure via headers |
| HIGH-01 | HIGH | WordPress | Missing security headers |
| HIGH-02 | HIGH | WordPress | Session cookies missing Secure flag |
| HIGH-03 | HIGH | WordPress | Author enumeration via URL parameter |
| HIGH-04 | HIGH | WordPress | wp-cron.php publicly accessible |
| HIGH-05 | HIGH | WordPress | install.php accessible |
| HIGH-06 | HIGH | MSS | API key documentation mismatch |
| MED-04 | MEDIUM | WordPress | license.txt/readme.html accessible |
| LOW-02 | LOW | WordPress | No robots.txt |

---

## Parallel Execution Plan

### Workstream 1: MSS Security Fixes
**Repository:** C:/Users/Admin/Documents/VS Projects/MSS
**Branch:** fix/mss-api-status-auth
**Estimated Time:** 30 minutes

**Tasks:**
1. Add authentication to /api/status endpoint
2. Ensure consistent auth behavior across all protected endpoints
3. Add tests for authentication
4. Update API documentation

**Files to modify:**
- `pkg/api/server.go` - Add auth middleware to /api/status route

---

### Workstream 2: WordPress Security Plugin (mu-plugin)
**Repository:** C:/Users/Admin/Documents/VS Projects/mss-suite
**Location:** docker/wordpress-security/mu-plugins/mss-security-hardening.php
**Estimated Time:** 45 minutes

**Tasks:**
1. Disable XML-RPC completely
2. Fix username enumeration (generic login errors)
3. Disable REST API user endpoints
4. Block author enumeration redirects
5. Remove WordPress version from HTML/meta
6. Remove version strings from CSS/JS URLs
7. Disable public wp-cron

**Code structure:**
```php
<?php
/**
 * Plugin Name: MSS Security Hardening
 * Description: Security hardening for CTF-ready WordPress
 * Version: 1.0.0
 */

// 1. Disable XML-RPC
add_filter('xmlrpc_enabled', '__return_false');
add_filter('xmlrpc_methods', '__return_empty_array');

// 2. Generic login error messages
add_filter('login_errors', function() {
    return 'Invalid username or password.';
});

// 3. Disable REST API user endpoints
add_filter('rest_endpoints', function($endpoints) {
    unset($endpoints['/wp/v2/users']);
    unset($endpoints['/wp/v2/users/(?P<id>[\d]+)']);
    return $endpoints;
});

// 4. Block author enumeration
add_action('template_redirect', function() {
    if (is_author() && !is_user_logged_in()) {
        wp_redirect(home_url(), 301);
        exit;
    }
});

// 5. Remove WordPress version
remove_action('wp_head', 'wp_generator');

// 6. Remove version from scripts/styles
add_filter('style_loader_src', 'mss_remove_version', 9999);
add_filter('script_loader_src', 'mss_remove_version', 9999);
function mss_remove_version($src) {
    return $src ? remove_query_arg('ver', $src) : $src;
}

// 7. Disable wp-cron via web
if (defined('DOING_CRON') && DOING_CRON) {
    if (!defined('WP_CLI') && php_sapi_name() !== 'cli') {
        // Only allow cron from CLI
        if (!isset($_SERVER['HTTP_HOST']) || strpos($_SERVER['HTTP_HOST'], 'localhost') === false) {
            exit('Cron disabled');
        }
    }
}
```

---

### Workstream 3: WordPress Server Configuration
**Repository:** C:/Users/Admin/Documents/VS Projects/mss-suite
**Location:** docker/wordpress-security/.htaccess
**Estimated Time:** 30 minutes

**Tasks:**
1. Add security headers
2. Block xmlrpc.php
3. Block install.php
4. Block wp-cron.php from external access
5. Block license.txt and readme.html
6. Add rate limiting rules (if mod_evasive available)

**Code structure:**
```apache
# Security Headers
<IfModule mod_headers.c>
    Header set X-Frame-Options "SAMEORIGIN"
    Header set X-Content-Type-Options "nosniff"
    Header set X-XSS-Protection "1; mode=block"
    Header set Referrer-Policy "strict-origin-when-cross-origin"
    Header unset X-Powered-By
    Header unset Server
</IfModule>

# Block XML-RPC
<Files xmlrpc.php>
    Order Deny,Allow
    Deny from all
</Files>

# Block install.php
<Files install.php>
    Order Deny,Allow
    Deny from all
</Files>

# Block wp-cron.php from external
<Files wp-cron.php>
    Order Deny,Allow
    Deny from all
    Allow from 127.0.0.1
    Allow from ::1
</Files>

# Block info files
<FilesMatch "^(license\.txt|readme\.html|wp-config-sample\.php)$">
    Order Deny,Allow
    Deny from all
</FilesMatch>
```

---

### Workstream 4: WordPress Static Files
**Repository:** C:/Users/Admin/Documents/VS Projects/mss-suite
**Location:** docker/wordpress-security/
**Estimated Time:** 15 minutes

**Tasks:**
1. Create robots.txt
2. Create security.txt
3. Update Docker compose to mount security files

**robots.txt:**
```
User-agent: *
Disallow: /wp-admin/
Disallow: /wp-includes/
Disallow: /wp-content/plugins/
Disallow: /wp-content/themes/
Disallow: /wp-login.php
Disallow: /xmlrpc.php
Disallow: /wp-cron.php
Allow: /wp-content/uploads/

Sitemap: /sitemap.xml
```

**security.txt (.well-known/security.txt):**
```
Contact: security@magnolia.dev
Expires: 2026-12-31T23:59:59Z
Preferred-Languages: en
Policy: https://magnolia.dev/security-policy
```

---

### Workstream 5: Docker Compose Update
**Repository:** C:/Users/Admin/Documents/VS Projects/mss-suite
**File:** docker/docker-compose.wordpress-test.yml
**Estimated Time:** 15 minutes

**Tasks:**
1. Mount mu-plugins directory to WordPress containers
2. Mount custom .htaccess
3. Mount robots.txt and security.txt
4. Add PHP configuration for session security

---

## Execution Order

```
┌─────────────────────────────────────────────────────────────┐
│                    PARALLEL EXECUTION                        │
├─────────────────┬─────────────────┬─────────────────────────┤
│  Workstream 1   │  Workstream 2   │  Workstream 3 + 4       │
│  MSS API Auth   │  WP mu-plugin   │  WP Server Config       │
│  (30 min)       │  (45 min)       │  (45 min)               │
│                 │                 │                         │
│  - server.go    │  - PHP plugin   │  - .htaccess            │
│  - tests        │  - 7 security   │  - robots.txt           │
│                 │    functions    │  - security.txt         │
└────────┬────────┴────────┬────────┴────────┬────────────────┘
         │                 │                 │
         └────────────────┬┴─────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  Workstream 5         │
              │  Docker Compose       │
              │  Integration          │
              │  (15 min)             │
              └───────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  Verification         │
              │  - Rebuild images     │
              │  - Start environment  │
              │  - Re-run security    │
              │    tests              │
              │  (30 min)             │
              └───────────────────────┘
```

---

## File Changes Summary

| Workstream | Repository | Files Changed |
|------------|------------|---------------|
| 1 | MSS | pkg/api/server.go |
| 2 | mss-suite | docker/wordpress-security/mu-plugins/mss-security-hardening.php |
| 3 | mss-suite | docker/wordpress-security/.htaccess |
| 4 | mss-suite | docker/wordpress-security/robots.txt, .well-known/security.txt |
| 5 | mss-suite | docker/docker-compose.wordpress-test.yml |

---

## Success Criteria

After fixes are applied:

- [ ] MSS /api/status returns 401 without valid API key
- [ ] XML-RPC blocked (403 or disabled)
- [ ] Login errors are generic
- [ ] /wp-json/wp/v2/users returns 404 or empty
- [ ] Author enumeration redirects to home
- [ ] No version strings in HTML source
- [ ] Security headers present on WordPress
- [ ] xmlrpc.php, install.php, wp-cron.php blocked
- [ ] robots.txt accessible
- [ ] security.txt accessible

---

## Rollback Plan

If issues occur:
1. Revert Docker compose to previous version
2. Remove mu-plugin mount
3. MSS: git checkout main -- pkg/api/server.go
4. Restart containers without security hardening

---

## Post-Fix Actions

1. Re-run full security test suite
2. Update CTF documentation with any intentional vulnerabilities
3. Document all security controls for participants
4. Create monitoring/logging for CTF event
