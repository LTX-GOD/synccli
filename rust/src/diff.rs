use crate::{DiffResult, DiffStatistics, FileDiff, FileMetadata};
use rayon::prelude::*;
use std::collections::HashMap;
use std::path::Path;

/// 差异计算器
pub struct DiffCalculator {
    // 可以添加配置选项
}

impl DiffCalculator {
    /// 创建新的差异计算器
    pub fn new() -> Self {
        Self {}
    }

    /// 计算文件差异
    pub fn calculate_differences(
        &self,
        source_files: &[FileMetadata],
        dest_files: &[FileMetadata],
    ) -> Result<DiffResult, String> {
        // 创建目标文件的哈希映射，以路径为键
        let dest_map: HashMap<String, &FileMetadata> = dest_files
            .iter()
            .map(|file| (self.normalize_path(&file.path), file))
            .collect();

        // 并行计算差异
        let differences: Vec<FileDiff> = source_files
            .par_iter()
            .filter_map(|source_file| self.compare_file(source_file, &dest_map))
            .collect();

        // 计算统计信息
        let statistics = self.calculate_statistics(source_files, dest_files, &differences);

        Ok(DiffResult {
            differences,
            statistics,
        })
    }

    /// 比较单个文件
    fn compare_file(
        &self,
        source_file: &FileMetadata,
        dest_map: &HashMap<String, &FileMetadata>,
    ) -> Option<FileDiff> {
        let normalized_path = self.normalize_path(&source_file.path);

        match dest_map.get(&normalized_path) {
            Some(dest_file) => {
                // 文件存在，检查是否需要更新
                if self.needs_update(source_file, dest_file) {
                    Some(FileDiff {
                        path: source_file.path.clone(),
                        operation: "update".to_string(),
                        source_hash: source_file.hash.clone(),
                        dest_hash: dest_file.hash.clone(),
                        size: source_file.size,
                    })
                } else {
                    // 文件相同，无需更新
                    None
                }
            }
            None => {
                // 文件不存在，需要创建
                Some(FileDiff {
                    path: source_file.path.clone(),
                    operation: "create".to_string(),
                    source_hash: source_file.hash.clone(),
                    dest_hash: String::new(),
                    size: source_file.size,
                })
            }
        }
    }

    /// 判断文件是否需要更新
    fn needs_update(&self, source_file: &FileMetadata, dest_file: &FileMetadata) -> bool {
        // 主要比较哈希值
        if source_file.hash != dest_file.hash {
            return true;
        }

        // 如果哈希相同但大小不同，也需要更新
        if source_file.size != dest_file.size {
            return true;
        }

        // 可以添加更多的比较条件，如修改时间等
        // 这里暂时只比较哈希和大小
        false
    }

    /// 标准化路径（处理不同操作系统的路径分隔符）
    fn normalize_path(&self, path: &str) -> String {
        // 将所有路径分隔符统一为 '/'
        path.replace('\\', "/")
    }

    /// 计算统计信息
    fn calculate_statistics(
        &self,
        source_files: &[FileMetadata],
        dest_files: &[FileMetadata],
        differences: &[FileDiff],
    ) -> DiffStatistics {
        let mut files_to_create = 0;
        let mut files_to_update = 0;
        let mut files_to_delete = 0;
        let mut total_size = 0;

        for diff in differences {
            match diff.operation.as_str() {
                "create" => files_to_create += 1,
                "update" => files_to_update += 1,
                "delete" => files_to_delete += 1,
                _ => {}
            }
            total_size += diff.size;
        }

        DiffStatistics {
            total_source_files: source_files.len(),
            total_dest_files: dest_files.len(),
            files_to_create,
            files_to_update,
            files_to_delete,
            total_size,
        }
    }

    /// 查找需要删除的文件（在目标目录中存在但源目录中不存在）
    pub fn find_files_to_delete(
        &self,
        source_files: &[FileMetadata],
        dest_files: &[FileMetadata],
    ) -> Vec<FileDiff> {
        // 创建源文件的哈希映射
        let source_map: HashMap<String, &FileMetadata> = source_files
            .iter()
            .map(|file| (self.normalize_path(&file.path), file))
            .collect();

        dest_files
            .par_iter()
            .filter_map(|dest_file| {
                let normalized_path = self.normalize_path(&dest_file.path);
                if !source_map.contains_key(&normalized_path) {
                    Some(FileDiff {
                        path: dest_file.path.clone(),
                        operation: "delete".to_string(),
                        source_hash: String::new(),
                        dest_hash: dest_file.hash.clone(),
                        size: dest_file.size,
                    })
                } else {
                    None
                }
            })
            .collect()
    }

    /// 按优先级排序差异列表
    pub fn sort_by_priority(&self, differences: &mut [FileDiff]) {
        differences.sort_by(|a, b| {
            // 优先级排序规则：
            // 1. 创建操作优先于更新操作
            // 2. 小文件优先于大文件
            // 3. 配置文件优先于其他文件

            let a_priority = self.get_file_priority(a);
            let b_priority = self.get_file_priority(b);

            b_priority.cmp(&a_priority) // 降序排列
        });
    }

    /// 获取文件优先级
    fn get_file_priority(&self, diff: &FileDiff) -> i32 {
        let mut priority = 0;

        // 操作类型优先级
        match diff.operation.as_str() {
            "create" => priority += 100,
            "update" => priority += 50,
            "delete" => priority += 10,
            _ => {}
        }

        // 文件大小优先级（小文件优先）
        if diff.size < 1024 * 1024 {
            // 小于1MB
            priority += 20;
        } else if diff.size < 10 * 1024 * 1024 {
            // 小于10MB
            priority += 10;
        }

        // 文件类型优先级
        let path = Path::new(&diff.path);
        if let Some(extension) = path.extension() {
            if let Some(ext_str) = extension.to_str() {
                match ext_str.to_lowercase().as_str() {
                    "json" | "yaml" | "yml" | "toml" | "ini" => priority += 30, // 配置文件
                    "md" | "txt" | "readme" => priority += 25,                  // 文档文件
                    "go" | "rs" | "py" | "js" | "ts" => priority += 20,         // 源代码文件
                    _ => {}
                }
            }
        }

        // 特殊文件名优先级
        if let Some(filename) = path.file_name() {
            if let Some(name_str) = filename.to_str() {
                match name_str.to_lowercase().as_str() {
                    "makefile" | "dockerfile" | "readme.md" => priority += 40,
                    "package.json" | "go.mod" | "cargo.toml" => priority += 35,
                    _ => {}
                }
            }
        }

        priority
    }
}

impl Default for DiffCalculator {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::FileMetadata;

    fn create_test_file(path: &str, hash: &str, size: i64) -> FileMetadata {
        FileMetadata {
            path: path.to_string(),
            hash: hash.to_string(),
            size,
            modified_time: "2023-01-01T00:00:00Z".to_string(),
            permissions: "0644".to_string(),
        }
    }

    #[test]
    fn test_calculate_differences_new_file() {
        let calculator = DiffCalculator::new();

        let source_files = vec![create_test_file("/test/new_file.txt", "hash1", 1024)];
        let dest_files = vec![];

        let result = calculator
            .calculate_differences(&source_files, &dest_files)
            .unwrap();

        assert_eq!(result.differences.len(), 1);
        assert_eq!(result.differences[0].operation, "create");
        assert_eq!(result.statistics.files_to_create, 1);
    }

    #[test]
    fn test_calculate_differences_updated_file() {
        let calculator = DiffCalculator::new();

        let source_files = vec![create_test_file("/test/file.txt", "hash_new", 1024)];
        let dest_files = vec![create_test_file("/test/file.txt", "hash_old", 1024)];

        let result = calculator
            .calculate_differences(&source_files, &dest_files)
            .unwrap();

        assert_eq!(result.differences.len(), 1);
        assert_eq!(result.differences[0].operation, "update");
        assert_eq!(result.statistics.files_to_update, 1);
    }

    #[test]
    fn test_calculate_differences_no_changes() {
        let calculator = DiffCalculator::new();

        let source_files = vec![create_test_file("/test/file.txt", "same_hash", 1024)];
        let dest_files = vec![create_test_file("/test/file.txt", "same_hash", 1024)];

        let result = calculator
            .calculate_differences(&source_files, &dest_files)
            .unwrap();

        assert_eq!(result.differences.len(), 0);
        assert_eq!(result.statistics.files_to_create, 0);
        assert_eq!(result.statistics.files_to_update, 0);
    }

    #[test]
    fn test_normalize_path() {
        let calculator = DiffCalculator::new();

        assert_eq!(
            calculator.normalize_path("C:\\test\\file.txt"),
            "C:/test/file.txt"
        );
        assert_eq!(
            calculator.normalize_path("/test/file.txt"),
            "/test/file.txt"
        );
    }

    #[test]
    fn test_find_files_to_delete() {
        let calculator = DiffCalculator::new();

        let source_files = vec![create_test_file("/test/keep.txt", "hash1", 1024)];
        let dest_files = vec![
            create_test_file("/test/keep.txt", "hash1", 1024),
            create_test_file("/test/delete.txt", "hash2", 512),
        ];

        let to_delete = calculator.find_files_to_delete(&source_files, &dest_files);

        assert_eq!(to_delete.len(), 1);
        assert_eq!(to_delete[0].operation, "delete");
        assert_eq!(to_delete[0].path, "/test/delete.txt");
    }
}
