# awsum

---

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

Thoes commands create a basic configuration for your awsum **and** potential future awscli installations.