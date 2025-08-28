local ignore_patterns = {
	".git",
	".svn",
	".hg",
	"node_modules",
	"target",
	"build",
	"dist",
	"__pycache__",
	".pytest_cache",
	".vscode",
	".idea",
	"*.swp",
	"*~",
	".DS_Store",
	"Thumbs.db",
	"desktop.ini",
	"*.log",
	"*.tmp",
	"*.temp",
	"*.cache"
}

local high_priority_extensions = {
	"lua", "go", "py", "rs", "md", "txt", "json", "yml", "yaml", "toml"
}

local low_priority_extensions = {
	"jpg", "jpeg", "png", "gif", "bmp", "mp4", "avi", "mov", "mkv", "mp3", "wav", "flac", "zip", "tar", "gz", "rar"
}

local function matches_pattern(path, pattern)
	if pattern:find("*") then
		local lua_pattern = pattern:gsub("*", ".*")
		return path:math(lua_pattern) ~= nil
	else
		return path:find(pattern, 1, true) ~= nil
	end
end

local function get_file_extension(file_path)
	return file_path:match("%.([^%.]+)$")
end

local function extension_in_list(extension, extension_list)
	if not extension then
		return false
	end

	extension = extension:lower()
	for _, ext in ipairs(extension_list) do
		if extension == ext:lower() then
			return true
		end
	end
	return false
end

function should_sync(file_path)
	for _, pattern in ipairs(ignore_patterns) do
		if matches_pattern(file_path, pattern) then
			return false
		end
	end

	local filename = file_path:match("([^/\\]+)$")

	if filename and filename:sub(1, 1) == "." then
		return false
	end

	if filename and (filename:match("%.bak") or filename:match("%.backup")) then
		return false
	end

	return true
end

function get_priority(file_path)
	local extension = get_file_extension(file_path)

	if extension_in_list(extension, high_priority_extensions) then
		return 10
	end

	if extension_in_list(extension, low_priority_extensions) then
		return 1
	end

	local filename = file_path:match("([^/\\]+)$")
	if filename then
		local config_file = {
			"Makefile", "makefile", "Dockerfile", "docker-compose.yml", "package.json", "go.mod",
			"Cargo.toml", "requirements.txt", "setup.py", "README.md", "LICENSE"
		}

		for _, config_file in ipairs(config_file) do
			if filename:lower() == config_file:lower() then
				return 15
			end
		end
	end
	if file_path:find("/src/") or file_path:find("/lib/") or file_path:find("/cmd/") then
		return 8
	end

	if file_path:find("/test/") or file_path:find("_test") then
		return 3
	end

	return 5
end

function custom_filter(files)
	local filtered = {}

	for _, file in ipairs(files) do
		if should_sync(file.path) then
			file.priority = get_priority(file.path)
			table.insert(filtered, file)
		end
	end

	table.sort(filtered, function(a, b)
		return a.priority > b.priority
	end)

	return filtered
end

function print_rules_info()
	print("=== FileSync CLI Rule Configuration ===")
	print("Number of ignore patterns: " .. #ignore_patterns)
	print("High priority extensions: " .. table.concat(high_priority_extensions, ", "))
	print("Low priority extensions: " .. table.concat(low_priority_extensions, ", "))
	print("=======================================")
end
