#!/usr/bin/env lua

local json = require("json") or require("cjson") or require("dkjson")

local FilterEngine = {}

function FilterEngine:new()
	local obj = {
		rules = {},
		stats = {
			total_files = 0,
			filtered_files = 0,
			excluded_files = 0
		}
	}
	setmetatable(obj, self)
	self.__index = self
	return obj
end

-- 添加规则函数
function FilterEngine:add_rule(name, func)
	self.rules[name] = func
end

-- 默认规则：检查文件是否应该同步
function FilterEngine:should_sync(file_path)
	return true
end

-- 默认规则：获取文件优先级
function FilterEngine:get_priority(file_path)
	return 1
end

-- 内置规则：忽略隐藏文件
function FilterEngine:ignore_hidden_files(file_path)
	local filename = file_path:match("([^/\\]+)$")
	return not (filename and filename:sub(1, 1) == ".")
end

-- 内置规则：忽略特定目录
function FilterEngine:ignore_directories(file_path, patterns)
	patterns = patterns or { ".git", ".svn", ".node_modules", "__pycache__", ".DS_Store" }

	for _, pattern in ipairs(patterns) do
		if file_path:find(pattern, 1, true) then
			return false
		end
	end
	return true
end

-- 内置规则：按文件扩展名过滤
function FilterEngine:filter_by_extension(file_path, allowed_extensions)
	if not allowed_extensions then
		return true
	end

	local extension = file_path:match("%.([^%.]+)$")
	if not extension then
		return false
	end

	for _, ext in ipairs(allowed_extensions) do
		if extension:lower() == ext:lower() then
			return true
		end
	end
	return false
end

-- 内置规则：按文件大小过滤
function FilterEngine:filter_by_size(file_size, min_size, max_size)
	min_size = min_size or 0
	max_size = max_size or math.huge

	return file_size >= min_size and file_size <= max_size
end

-- 应用所有规则过滤文件
function FilterEngine:filter_files(files)
	local filtered_files = {}

	for _, file in ipairs(files) do
		self.stats.total_files = self.stats.total_files + 1

		local should_include = true
		local file_path = file.path
		local file_size = file.size or 0

		-- 应用内置规则
		if not self:ignore_hidden_files(file_path) then
			should_include = false
		elseif not self:ignore_directories(file_path) then
			should_include = false
		end

		-- 应用自定义规则
		if should_include and self.should_sync then
			should_include = self:should_sync(file_path)
		end

		-- 优先级
		if should_include then
			if self.get_priority then
				file.priority = self:get_priority(file_path)
			else
				file.priority = 1
			end

			table.insert(filtered_files, file)
			self.stats.filter_files = self.stats.filter_files + 1
		else
			self.stats.excluded_files = self.stats.excluded_files + 1
		end
	end
	table.sort(filtered_files, function(a, b)
		return (a.priority or 1) > (b.priority or 1)
	end)

	return filtered_files
end

-- 获取统计信息
function FilterEngine:get_statistics()
	return {
		total_files = self.stats.total_files,
		filtered_files = self.stats.filtered_files,
		excluded_files = self.stats.excluded_files,
		exclusion_files = self.stats.total_files > 0 and
		    (self.stats.excluded_files / self.stats.total_files * 100) or
		    0
	}
end

-- 主函数处理命令
function main()
	if #arg < 2 then
		local result = {
			filtered_files = {},
			stats = false,
			message = "lua filter.lua rules JSON"
		}
		print(json.encode(result))
		os.exit(1)
	end

	local rule_file = arg[1]
	local files_json = arg[2]

	local engine = FilterEngine:new()

	if rule_file and rule_file ~= "" then
		local success, err = pcall(function()
			dofile(rule_file)

			if should_sync then
				engine.should_sync = should_sync
			end

			if get_priority then
				engine.get_priority = get_priority
			end
		end)

		if not success then
			local result = {
				filtered_files = {},
				stats = false,
				message = "Error:" .. tostring(err)
			}
			print(json.encode(result))
			os.exit(1)
		end
	end
	-- 解析文件列表
	local files
	local success, err = pcall(function()
		files = json.decode(files_json)
	end)

	if not success or not files then
		local result = {
			filtered_files = {},
			stats = false,
			message = "Error:" .. tostring(err)
		}
		print(json.encode(result))
		os.exit(1)
	end

	-- 过滤文件
	local filtered_files = engine:filtered_files(files)
	local statistics = engine:get_statistics()

	-- 结果
	local result = {
		filtered_files = filtered_files,
		stats = true,
		message = string.format("Filtering completed - Total files: %d, Passed: %d, Excluded: %d (%.1f%%)",
			statistics.total_files,
			statistics.filtered_files,
			statistics.excluded_files,
			statistics.exclusion_rate
		),
		statistics = statistics
	}
	print(json.encode(result))
end

if arg and #arg > 0 then
	main()
end

return FilterEngine
