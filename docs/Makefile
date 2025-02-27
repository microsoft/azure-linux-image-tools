.PHONY: prereqs
prereqs:
	gem install jekyll bundler

.PHONY: env
env:
	bundle install

.PHONY: serve
serve:
	bundle exec jekyll serve

.PHONY: build
build:
	bundle exec jekyll build
	bundle exec htmlproofer --assume_extension '.html' ./_site

update-dependencies:
	bundle update
