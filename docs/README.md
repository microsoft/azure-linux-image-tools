# Testing github pages locally

This includes documentation required to test local changes related to github
pages. Current repo github pages leverages  `jekyll-theme-minimal` theme.

* Install dependencies

```bash
sudo apt update
sudo apt install ruby-full build-essential zlib1g-dev
```

* Create Gemfile under `docs` directory

```bash
# Create the Gemfile
cat <<EOF > Gemfile
source "https://rubygems.org"

gem "jekyll", "~> 4.3.0"
gem "webrick"
gem "jekyll-theme-minimal"
EOF
```

* Install Bundle & dependencies

```bash
sudo gem install jekyll bundler
cd docs
bundle install
```

* Update `_config.yml`

```yaml
#baseurl: "/azure-linux-image-tools"
```

* Serve the site locally

```bash
bundle exec jekyll serve
```

You can adjust the --host and --port parameters as needed. By default, the site
will be available at http://localhost:4000
