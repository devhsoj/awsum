# awsum

a fun CLI tool for working with AWS infra (cross-platform)

## Installation
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
awsum configure
```

These commands create a basic configuration for your awsum **and** potential future awscli installations.

## Usage

All AWS operations triggered by AWS service clients created by awsum are logged to files in a `awsum` directory created in the `~/.aws` directory.

* `~/.aws/awsum/awsum-global-aws-log-output` for a record of all operations done by executions of awsum.
* `~/.aws/awsum/awsum-session-aws-log-output-YYYY-MM-DD__HH-mm-SS` for operations grouped by individual executions of awsum.

To get a description of awsum and how to use its commands and sub-commands:
```shell
awsum --help
```

**(Example)** To get help with commands and sub-commands:
```shell
awsum instance --help
```

### Real-World Examples

Get a list of all instances in csv:
```shell
awsum instance list --format csv
```

Sequentially open a secure shell (SSH) to every instance with a name matching "worker":
```shell
awsum instance shell --name "worker"
```

Get the free disk space of every ec2 instance with a name matching "website" over SSH:
```shell
awsum instance shell --name website "df -h"
```

Basic app deployment w/ load-balancing (Amazon Linux example):

**Note:** awsum does not modify any non-command-related resources to prevent breaking existing infrastructure.

**Note 2:** This is actually an exact replica of the demo deployment done by awsum (on two t2.nano instances) [NGINX Demo](https://awsum.levelshatter.com/).

**Note 3:** Please, please, please, properly secure your CI/CD platforms, your instances, and lock down the users awsum will authenticate as, you do not want to give fully privileged RCE to anyone/and or service making code changes...

```shell
# basic deployment logic

awsum instance shell --name demo "docker rm nginx --force"
awsum instance shell --name demo "docker run -d -p 80:80 --name nginx nginxdemos/hello"

"
load balance an http service running on port 80 on
instances matching the name 'demo' using https with
a certificate from ACM and with a domain pointing to it.
"

awsum instance load-balance \
    --service "nginx-demo" \
    --name demo \
    --port 443:80 \
    --protocol https:http \
    --certificate "levelshatter.com" \
    --domain "awsum.levelshatter.com"
```

awsum really shines when used in CI/CD processes.