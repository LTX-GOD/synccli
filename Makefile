# FileSync CLI 构建脚本

.PHONY: all build clean install test deps

# 默认目标
all: deps build

# 安装依赖
deps:
	@echo "安装Go依赖..."
	go mod tidy
	@echo "安装Python依赖..."
	pip3 install -r python/requirements.txt
	@echo "构建Rust库..."
	cd rust && cargo build --release

# 构建项目
build: deps
	@echo "构建Go CLI..."
	go build -o bin/synccli ./cmd/synccli
	@echo "构建完成！"

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -rf bin/
	cd rust && cargo clean
	@echo "清理完成！"

# 安装到系统
install: build
	@echo "安装synccli到系统..."
	cp bin/synccli /usr/local/bin/
	@echo "安装完成！"

# 运行测试
test:
	@echo "运行Go测试..."
	go test ./...
	@echo "运行Python测试..."
	python3 -m unittest discover python/tests
	@echo "运行Rust测试..."
	cd rust && cargo test
	@echo "测试完成！"

# 创建发布包
release: clean build
	@echo "创建发布包..."
	mkdir -p release
	cp bin/synccli release/
	cp -r lua/ release/
	cp -r scripts/ release/
	cp README.md release/
	tar -czf release/synccli-$(shell date +%Y%m%d).tar.gz -C release .
	@echo "发布包创建完成！"
