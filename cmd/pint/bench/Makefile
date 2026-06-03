REVISION := 5cc052fc0a2a641ca3863dc748c32e1a1a552f4f

.PHONE: fetch
fetch:
	curl -sL -o archive.tar.gz https://github.com/samber/awesome-prometheus-alerts/archive/$(REVISION).tar.gz
	tar -xf archive.tar.gz
	rm -fr rules
	mv awesome-prometheus-alerts-$(REVISION)/dist/rules rules
	rm -fr awesome-prometheus-alerts-$(REVISION) archive.tar.gz
