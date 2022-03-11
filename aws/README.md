# Setup instructions

- Create a VPC with private and public subnet
- Create a NAT gateway to provide external connectivity for the instances
- Create a custom AMI based on Ubuntu 20.04 having kata-agent and other dependencies.
- Create an EC2 launch template named "kata". 


# Running cloud-api-adaptor

```
cloud-api-adaptor-aws aws \
    -aws-access-key-id ${AWS_ACCESS_KEY_ID} \
    -aws-secret-key ${AWS_SECRET_ACCESS_KEY} \
    -aws-region ${AWS_REGION} \
    -pods-dir /run/peerpod/pods \
    -socket /run/peerpod/hypervisor.sock
```

