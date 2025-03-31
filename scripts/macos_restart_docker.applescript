-- Applescript to close and reopen Docker Desktop on macOS
-- run with `osascript scripts/macos_restart_docker.applescript`

set dockerAppName to "Docker Desktop"
set extraDelay to 1

if appIsRunning(dockerAppName) then
    log "Stopping " & dockerAppName
    tell application dockerAppName to quit

    -- wait until it's fully closed
    repeat until not appIsRunning(dockerAppName)
        delay 1
    end repeat
    delay extraDelay
else
    log dockerAppName & " is not running"
end if

-- start docker again
log "Starting " & dockerAppName
do shell script "open --background -a '" & dockerAppName & "'"

-- wait until it's fully running
log "Waiting for " & dockerAppName & " to start"
repeat until appIsRunning(dockerAppName)
	delay 1
end repeat
delay extraDelay

log "Restarted " & dockerAppName

on appIsRunning(appName)
	tell application "System Events" to (name of processes) contains appName
end appIsRunning
