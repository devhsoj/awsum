# awsum

a fun (cross-platform) CLI tool for working with AWS infra

## Getting Started
*Required:* [Go 1.25](https://go.dev/dl)

**Installing from source:**
```shell
git clone https://github.com/levelshatter/awsum
cd awsum/
go install
```

**Installing via go install:**

```shell
go install github.com/levelshatter/awsum@latest
```

**Installing via packaged release:**
[Releases](https://github.com/levelshatter/awsum/releases)

### Configuring

If you have [awscli](https://aws.amazon.com/cli/) installed & configured, you are already good to go!

If not, then you can do the following:

```shell
mkdir ~/.aws

echo "[default]
region = YOUR_REGION" > ~/.aws/config

echo "[default]
aws_access_key_id = YOUR_AWS_ACCESS_KEY_ID
aws_secret_access_key = YOUR_AWS_SECRET_ACCESS_KEY" > ~/.aws/credentials
```

These commands create a basic configuration for your awsum **and** potential future awscli installations.

---

## Usage

To get a description of awsum and how to use its commands and sub-commands:
```shell
awsum help
```

**(Examples)** To get help with commands and sub-commands:
```shell
awsum instance help
```

```shell
awsum instance shell --help
```

### Real-World Examples

Get a list of all instances
```shell
awsum instance list --format csv
```

Get the free disk space of every ec2 instance with a name matching "website" (we are assuming every instance is linux in this case):
```shell
awsum instance shell --name website "df -h"
```

Run a ping test to CloudFlare's DNS service on all instances matching the name "api-server":
```shell
awsum instance shell --name "api-server" "ping 1.1.1.1 -c 3"
```