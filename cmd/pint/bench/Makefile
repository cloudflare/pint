REVISION := 9f5c641bddde827d25c0284134c16a5ac3b94503

.PHONE: fetch
fetch:
	curl -sL -o archive.tar.gz https://github.com/samber/awesome-prometheus-alerts/archive/$(REVISION).tar.gz
	tar -xf archive.tar.gz
	rm -fr rules
	mv awesome-prometheus-alerts-$(REVISION)/dist/rules rules
	rm -fr awesome-prometheus-alerts-$(REVISION) archive.tar.gz
