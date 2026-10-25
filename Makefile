# 顶层 Makefile：常用入口的薄壳，真正的活儿在各自的脚本里（口径同 phpo 顶层 Makefile）。
# 版本号只有一个家：仓库根 VERSION 文件，存「下一个要打的号」。取号与推格都只发生在这里——
#   不带 VERSION 出包 → 从文件取号，**全部包打成之后**推一格（包打成＝set -e 全程没红）；
#   带 VERSION=x      → 四条线都用 x，打完文件照常推进到「x 的下一格」，保证自动号不撞车。
#   （推格放在 recipe 最后一行，前面的打包脚本任何一个失败，make 当场停，号不会被白白消耗。）
# 四种包：deb／rpm／AppImage（build/linux/）与 Windows zip（build/windows/）。
#   `make package` 一条龙用同一个号出四件；要单出某一种就 make deb / rpm / appimage / win。
SHELL := /bin/bash

VERSION_FILE := VERSION
ifeq ($(strip $(VERSION)),)
CURVER := $(shell cat $(VERSION_FILE) 2>/dev/null || echo 0.0.0-m5)
else
CURVER := $(VERSION)
endif
NEXTVER := $(shell v='$(CURVER)'; [[ $$v =~ ^(.*[^0-9])([0-9]+)$$ ]] \
	&& printf '%s%d' "$${BASH_REMATCH[1]}" "$$(( $${BASH_REMATCH[2]} + 1 ))" \
	|| printf '%s.1' "$$v")

.PHONY: package deb rpm appimage win frontend build test vet fmt check clean

package:
	bash build/linux/make-deb.sh $(CURVER)
	bash build/linux/make-rpm.sh $(CURVER)
	bash build/linux/make-appimage.sh $(CURVER)
	bash build/windows/make-win.sh $(CURVER)
	printf '%s\n' $(NEXTVER) > $(VERSION_FILE)
	@echo "下一个版本号已写进 $(VERSION_FILE)：$(NEXTVER)"

deb:
	bash build/linux/make-deb.sh $(CURVER)
	printf '%s\n' $(NEXTVER) > $(VERSION_FILE)

rpm:
	bash build/linux/make-rpm.sh $(CURVER)
	printf '%s\n' $(NEXTVER) > $(VERSION_FILE)

appimage:
	bash build/linux/make-appimage.sh $(CURVER)
	printf '%s\n' $(NEXTVER) > $(VERSION_FILE)

win:
	bash build/windows/make-win.sh $(CURVER)
	printf '%s\n' $(NEXTVER) > $(VERSION_FILE)

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
