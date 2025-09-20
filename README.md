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

awsum uses the same exact configuration the [awscli](https://aws.amazon.com/cli/) tool uses (since we use the client library already) to keep environments less messy.

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

## Usage

To get a description of awsum and how to use its commands and sub-commands:
```shell
awsum help
```

**(Example)** To get help with commands and sub-commands:
```shell
awsum instance
```

### Real-World Examples

Get a list of all instances
```shell
awsum instance list --format csv
```

Get the free disk space of every ec2 instance with a name matching "website" (assuming nix-like system):
```shell
awsum instance shell --name website "df -h"
```

Basic app deployment w/ load-balancing (Amazon Linux):
```shell
#!/bin/bash

instance_name_filter=$1

if [[ -z $instance_name_filter ]] then
	echo "usage: $0 [INSTANCE NAME FILTER]"
	exit 1
fi

# basic deployment

awsum instance shell --name "$instance_name_filter" "sudo yum install docker -y"
awsum instance shell --name "$instance_name_filter" "sudo service docker start"
awsum instance shell --name "$instance_name_filter" "sudo usermod -aG docker ec2-user"
awsum instance shell --name "$instance_name_filter" "docker rm nginx --force"
awsum instance shell --name "$instance_name_filter" "docker run -d -p 80:80 --name nginx nginxdemos/hello"

# load balancing

awsum instance load-balance --service "nginx-demo" --name "$instance_name_filter" --port 80 --protocol http
```

awsum really shines when used in CI/CD processes.