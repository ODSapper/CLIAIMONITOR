$now = (Get-Date).ToString('yyyy-MM-ddTHH:mm:ss.fffffffzzz')
$path = "C:\Users\Admin\Documents\VS Projects\CLIAIMONITOR\data\state.json"
$json = Get-Content $path -Raw | ConvertFrom-Json
$json.session_stats.total_agents_spawned = 0
$json.session_stats.total_tokens_used = 0
$json.session_stats.total_estimated_cost = 0
$json.session_stats.completed_tasks = 0
$json.session_stats.session_started_at = $now
$json | ConvertTo-Json -Depth 10 | Set-Content $path
Write-Host "Session reset to $now"
