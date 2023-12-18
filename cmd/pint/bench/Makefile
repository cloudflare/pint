REVISION := 31a27fb9e0e778bd8fe6097aa58c8ea598fe9cec

.PHONE: fetch
fetch:
	curl -sL -o archive.tar.gz https://github.com/samber/awesome-prometheus-alerts/archive/$(REVISION).tar.gz
	tar -xf archive.tar.gz
	rm -fr rules
	mv awesome-prometheus-alerts-$(REVISION)/dist/rules rules
	rm -fr awesome-prometheus-alerts-$(REVISION) archive.tar.gz
