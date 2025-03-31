-- take initial snapshot of running processes
tell application "System Events" to set initialProcesses to name of processes

repeat
	tell application "System Events" to set currentProcesses to name of processes
	set newProcesses to {}
	repeat with p in currentProcesses
		if p is not in initialProcesses then set end of newProcesses to p
	end repeat
	
	-- if (count of newProcesses) > 0 then
    set currentTime to do shell script "date '+%Y-%m-%d %H:%M:%S'"
    set newProcessesStr to my listToString(newProcesses)
    log currentTime & ": " & newProcessesStr
	-- end if
	
	delay 1
end repeat

on listToString(aList)
	set textStr to ""
	repeat with anItem in aList
		set textStr to textStr & anItem & ", "
	end repeat
	-- remove the last comma and space
	if (length of textStr) > 0 then
		set textStr to text 1 through ((length of textStr) - 2) of textStr
	end if
	return textStr
end listToString

