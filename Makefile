# 顶层 Makefile：常用入口的薄壳，真正的活儿在各自的脚本/工具里（口径同 phpo 顶层 Makefile）。
# 出包命令全拼是 bash build/linux/make-deb.sh <版本>；这里的 package 把它收短，
# 版本号不传就用默认（与脚本里的默认同一个号，不抄第二份——脚本自己兜底）。

VERSION ?=

.PHONY: package frontend build test vet fmt clean

# 出 deb 包：make package            → 用脚本默认版本
#           make package VERSION=0.0.0-m6
package:
	bash build/linux/make-deb.sh $(VERSION)

# 前端依赖与产物（首次 / 依赖变了才需要）
frontend:
	cd frontend && npm install && npm run build

build:
	go build -tags gtk3 ./...

test:
	go test -tags gtk3 ./...

vet:
	go vet -tags gtk3 ./...

# 全套检查号（CI 同一口径的本地版）：测试 + vet + gofmt + Windows 交叉编译
check: test vet
	gofmt -l . | grep -q . && echo "有文件需要 gofmt" && exit 1 || true
	GOOS=windows go build ./...

clean:
	rm -rf build/out frontend/dist
