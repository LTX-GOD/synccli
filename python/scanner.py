import hashlib
import json
import os
import re
import stat
import sys
from ast import walk
from datetime import datetime
from ntpath import isdir, isfile, islink
from pathlib import *
from tabnanny import verbose
from textwrap import indent
from typing import *
from warnings import filters


class FileMetadate:
    # 文件元数据类
    def __init__(self,path:str,hash_value:str="",size:int=0,modified_time:datetime= None , permissions:str=""):
        self.path=path
        self.hash=hash_value
        self.size=size
        self.modified_time=modified_time or datetime.now()
        self.permissions=permissions

    # 转成字典
    def to_dict(self)->Dict[str,Any]:
        return{
            "path":self.path,
            "hash":self.hash,
            "size":self.size,
            "modified_time":self.modified_time.strftime("%Y-%m-%dT%H:%H:%M:%SZ"),
            "permissions":self.permissions
        }

import hashlib
import os
import stat
import sys
from datetime import datetime
from typing import Any, Dict, List


class FileScanner:
    # 文件扫描器
    def __init__(self, verbose: bool = False):
        self.verbose = verbose
        self.scanned_files = 0
        self.total_size = 0

    # 计算hash
    def calculate_hash(self, file_path: str) -> str:
        try:
            hash_sha256 = hashlib.sha256()
            with open(file_path, "rb") as f:
                for chunk in iter(lambda: f.read(4096), b""):
                    hash_sha256.update(chunk)
            return hash_sha256.hexdigest()
        except (IOError, OSError) as e:
            if self.verbose:
                print(f"Error: Can't get hash of file {file_path}: {e}", file=sys.stderr)
            return ""

    # 获取文件权限
    def get_file_permissions(self, file_path: str) -> str:
        try:
            file_stat = os.stat(file_path)
            return oct(stat.S_IMODE(file_stat.st_mode))
        except (IOError, OSError):
            return "0644"

    # 扫描单个文件并且返回元数据
    def scan_file(self, file_path: str) -> 'FileMetadate':
        try:
            file_stat = os.stat(file_path)
            modified_time = datetime.fromtimestamp(file_stat.st_mtime)
            permissions = self.get_file_permissions(file_path)

            self.scanned_files += 1
            self.total_size += file_stat.st_size

            return FileMetadate(
                path=file_path,
                hash_value=self.calculate_hash(file_path),  # 计算文件的hash
                size=file_stat.st_size,
                modified_time=modified_time,
                permissions=permissions
            )
        except (IOError, OSError) as e:
            if self.verbose:
                print(f"Error: Can't scan the file {file_path}: {e}", file=sys.stderr)
            return None

    # 扫描路径并且返回元数据列表
    def scan_path(self, path: str) -> List['FileMetadate']:
        files_metadata = []

        if not os.path.exists(path):
            if self.verbose:
                print(f"Error: Don't have this path: {path}", file=sys.stderr)
            return files_metadata

        if os.path.isfile(path):
            file_metadata = self.scan_file(path)
            if file_metadata:
                files_metadata.append(file_metadata)
            return files_metadata

        if os.path.isdir(path):
            return self.scan_directory(path)

        if self.verbose:
            print(f"Error: is not file or dir: {path}", file=sys.stderr)

        return files_metadata

    # 递归扫描目录
    def scan_directory(self, directory_path: str) -> List['FileMetadate']:
        files_metadata = []

        if not os.path.exists(directory_path):
            if self.verbose:
                print(f"Error: the file is nothing: {directory_path}", file=sys.stderr)
            return files_metadata

        if not os.path.isdir(directory_path):
            if self.verbose:
                print(f"Error: this is not dir: {directory_path}", file=sys.stderr)
            return files_metadata

        try:
            for root, dirs, files in os.walk(directory_path):
                dirs[:] = [d for d in dirs if not d.startswith('.')]  # 排除隐藏文件夹
                for file_name in files:
                    if file_name.startswith('.'):  # 排除隐藏文件
                        continue
                    file_path = os.path.join(root, file_name)
                    if os.path.islink(file_path):  # 排除符号链接
                        continue

                    file_metadata = self.scan_file(file_path)
                    if file_metadata:
                        files_metadata.append(file_metadata)
                    if self.verbose and self.scanned_files % 100 == 0:
                        print(f"Scanned {self.scanned_files} files", file=sys.stderr)
        except (IOError, OSError) as e:
            if self.verbose:
                print(f"Error: Scanning issue with {directory_path}: {e}", file=sys.stderr)

        return files_metadata

    # 获取扫描统计信息
    def get_statistics(self) -> Dict[str, Any]:
        return {
            "scanned_files": self.scanned_files,
            "total_size": self.total_size,
            "total_size_mb": round(self.total_size / (1024 * 1024), 2)
        }

def main():
    if len(sys.argv)<3:
        result={
            "source_files":[],
            "dest_files":[],
            "status":False,
            "message":"python3 scanner.py <> <> [--verbose]"
        }
        print(json.dumps(result,ensure_ascii=False,indent=2))
        sys.exit(1)

    source_path=sys.argv[1]
    dest_path=sys.argv[2]
    verbose="--verbose" in sys.argv

    try:
        scanner=FileScanner(verbose=verbose)

        if verbose:
            print(f"Starting scan the path: {source_path}",file=sys.stderr)

        source_files=scanner.scan_path(source_path)
        source_stats=scanner.get_statistics()

        if verbose:
            print(f"Scanned over: {source_stats['scanned_files']} , "
                  f"{source_stats['total_size_mb']} MB", file=sys.stderr)
            print(f"Starting to scan the path: {dest_path}", file=sys.stderr)
        # 扫描统计信息
        scanner.scanned_files=0 
        scanner.total_size=0 

        # 扫描目标路径
        dest_files=scanner.scan_path(dest_path)
        dest_stats=scanner.get_statistics()

        if verbose:
            print(f"Scanned over: {dest_stats['scanned_files']} files, "
                  f"{dest_stats['total_size_mb']} MB", file=sys.stderr)

        # 结果
        result={
            "source_files":[f.to_dict() for f in source_files],
            "dest_files":[f.to_dict() for f in dest_files],
            "status":True,
            "message":f"Scanned end :{len(source_files)},the dest:{len(dest_files)}",
            "statistics":{
                "source":source_stats,
                "dest":dest_stats
            }
        }

        print(json.dumps(result,ensure_ascii=False,indent=2))
    except Exception as e:
        result={
            "source_files":[],
            "dest_files":[],
            "status":False,
            "message":f"Error with {str(e)}"
        }
        print(json.dumps(result,ensure_ascii=False,indent=2))
        sys.exit(1)
if __name__=="__main__":
    main()
