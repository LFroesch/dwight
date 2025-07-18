build:
	go build -o dwight
cp:
	cp dwight ~/.local/bin/
	
install: build cp