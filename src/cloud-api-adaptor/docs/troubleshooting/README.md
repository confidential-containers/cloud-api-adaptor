# Troubleshooting

The official documentation for Confidential Containers is currently under re-work. An archived version of the Peer pods troubleshooting guide can be found [here](https://github.com/confidential-containers/confidentialcontainers.org/blob/7a861f4d26c48100004d2c6e72298f2592cc04c0/content/en/docs/cloud-api-adaptor/troubleshooting.md).

## Debugging peer pod VM images

Sometimes when debugging peer pod VM images (e.g. during development), it is helpful to override the cloud-api-adaptor's
default behaviour, which is to delete peer pod VM instances when it received the StopVM request. For example if there is
a configuration issue in the peer pod VM, and it fails to connect to the cloud-api-adaptor process, then it will be
automatically deleted, removing the ability to debug this problem. To override this behaviour, set
`PEERPODS_DEVELOPER_MODE` to `true` in the peer-pods-cm configmap.
> [!NOTE]
> In developer mode, the `PEERPODS_LIMIT_PER_NODE` defaults to 1 to avoid Kubernetes creating multiple instances of
> pods that don't work. If you wish to override this then please set the value in peer-pods-cm.
