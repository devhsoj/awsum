# awsum

a fun CLI tool for working with AWS infra (cross-platform)

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

All AWS operations triggered by AWS service clients created by awsum are logged to files in a `awsum` directory created in the `~/.aws` directory.

* `~/.aws/awsum/awsum-global-aws-log-output` for a record of all operations done by executions of awsum.
* `~/.aws/awsum/awsum-session-aws-log-output-YYYY-MM-DD__HH-MM-SS` (filename format) for operations grouped by individual executions of awsum.

To get a description of awsum and how to use its commands and sub-commands:
```shell
awsum help
```

**(Example)** To get help with commands and sub-commands:
```shell
awsum instance
```

### Real-World Examples

Get a list of all instances in csv:
```shell
awsum instance list --format csv
```

Sequentially open a secure shell (SSH) to every instance with a name matching "game-server":
```shell
awsum instance shell --name "game-server"
```

Get the free disk space of every ec2 instance with a name matching "website" over SSH:
```shell
awsum instance shell --name website "df -h"
```

Basic app deployment w/ load-balancing (Amazon Linux example):

**Note:** awsum does not modify any non-awsum related security groups, the security group(s) attached to your instances,
your Route 53 records, or any of your certificates. awsum is designed this way to prevent breaking or insecure configurations
to your already existing or newly created infrastructure.

```shell
# basic deployment

awsum instance shell --name demo "sudo yum install docker -y"
awsum instance shell --name demo "sudo service docker start"
awsum instance shell --name demo "sudo usermod -aG docker ec2-user"
awsum instance shell --name demo "docker rm nginx --force"
awsum instance shell --name demo "docker run -d -p 80:80 --name nginx nginxdemos/hello"

# load balancing

awsum instance load-balance --service "nginx-demo" --name demo --port 443:80 --protocol https --certificate "awsum.levelshatter.com"
```

awsum really shines when used in CI/CD processes.