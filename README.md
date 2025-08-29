# FileSync CLI: Multi-Language File Synchronization tool

This is a small file synchronization tool,

created as a personal practice project. 

It is written using Golang (CLI interface), Rust (encryption), Python (file scanning), and Lua (rules module).

## Architecture Design

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Go CLI Entry  │───▶│  Python Scanner │───▶│  Lua Rule Engine│
│ Command/Concurrency│    │ Metadata/Hashing │    │ File Filter Rules│
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Rust Performance│    │ SSH Remote Conn │    │ Progress Tracking│
│ Diff/Encrypt/Comp│    │ SFTP File Transfer│    │ Real-time Updates│
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

# Usage Guide

## Local Synchronization
```bash
# Basic sync
./bin/synccli sync /path/to/source /path/to/dest

# Use custom rules
./bin/synccli sync /path/to/source /path/to/dest --rule lua/rules.lua

# Enable encryption and compression
./bin/synccli sync /path/to/source /path/to/dest --encrypt --compress
```

## Remote Synchronization
```bash
# SSH remote sync
./bin/synccli remote sync /local/path /remote/path \
  --host example.com \
  --user username \
  --key ~/.ssh/id_rsa

# Use password authentication
./bin/synccli remote sync /local/path /remote/path \
  --host example.com \
  --user username \
  --password

# Configuration management
./bin/synccli remote config save myserver \
  --host example.com \
  --user username \
  --key ~/.ssh/id_rsa

./bin/synccli remote sync /local/path /remote/path --config myserver
```

## Lua Rule Scripts

Create custom synchronization rules:

```lua
-- lua/rules.lua

-- Define files to sync
function should_sync(file_path)
    -- Ignore .git directory
    if string.find(file_path, "%.git/") then
        return false
    end
    
    -- Ignore temporary files
    if string.find(file_path, "%.tmp$") or string.find(file_path, "%.temp$") then
        return false
    end
    
    -- Ignore large files (over 100MB)
    local file_size = get_file_size(file_path)
    if file_size > 100 * 1024 * 1024 then
        return false
    end
    
    return true
end

-- Define sync priority
function get_priority(file_path)
    -- Config files have highest priority
    if string.find(file_path, "%.config$") or string.find(file_path, "%.json$") then
        return 10
    end
    
    -- Source code files have medium priority
    if string.find(file_path, "%.go$") or string.find(file_path, "%.py$") then
        return 5
    end
    
    -- Other files have low priority
    return 1
end
```

# Contact

- Send email to: 2924465428@qq.com
