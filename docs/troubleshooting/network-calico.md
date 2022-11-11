# Some tips for digging network issues between Worker and PeerPod

## Check calico node status on each worker
If you're using Calico CNI, check the BGP is set correctly

```
calicoctl node status
```

Result should be similar as following:
```
Calico process is running.

IPv4 BGP status
+--------------+-------------------+-------+------------+-------------+
| PEER ADDRESS |     PEER TYPE     | STATE |   SINCE    |    INFO     |
+--------------+-------------------+-------+------------+-------------+
| 10.242.0.4   | node-to-node mesh | up    | 2022-08-22 | Established |
| 10.242.0.5   | node-to-node mesh | up    | 2022-08-22 | Established |
| 10.242.0.9   | node-to-node mesh | up    | 2022-08-22 | Established |
+--------------+-------------------+-------+------------+-------------+

IPv6 BGP status
No IPv6 peers found.
```

## Determine the network connections
Check the IP and Ports connections, like:

```
# netstat -antp
Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:179             0.0.0.0:*               LISTEN      37164/bird
tcp        0      0 127.0.0.53:53           0.0.0.0:*               LISTEN      1216/systemd-resolv
tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      803/sshd
tcp        0      0 0.0.0.0:31799           0.0.0.0:*               LISTEN      6558/kube-proxy
tcp        0      0 172.20.0.1:2040         0.0.0.0:*               LISTEN      39592/haproxy
tcp        0      0 0.0.0.0:31676           0.0.0.0:*               LISTEN      6558/kube-proxy
tcp        0      0 127.0.0.1:18144         0.0.0.0:*               LISTEN      12545/containerd
tcp        0      0 10.242.0.10:10210       0.0.0.0:*               LISTEN      12545/containerd
tcp        0      0 127.0.0.1:10248         0.0.0.0:*               LISTEN      6500/kubelet
tcp        0      0 127.0.0.1:10249         0.0.0.0:*               LISTEN      6558/kube-proxy
tcp        0      0 127.0.0.1:9098          0.0.0.0:*               LISTEN      36355/calico-typha
tcp        0      0 127.0.0.1:9099          0.0.0.0:*               LISTEN      36888/calico-node
tcp        0      0 10.242.0.10:13401       172.21.0.1:443          ESTABLISHED 36355/calico-typha
tcp        0      0 127.0.0.1:52255         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 172.20.0.1:2040         172.20.0.1:53317        ESTABLISHED 39592/haproxy
tcp        0      0 10.242.0.10:3211        141.125.67.34:31637     ESTABLISHED 39592/haproxy
tcp        0      0 127.0.0.1:9098          127.0.0.1:65283         TIME_WAIT   -
tcp        0      0 172.20.0.1:2040         10.242.0.10:13401       ESTABLISHED 39592/haproxy
tcp        0      0 10.242.0.10:3207        141.125.67.34:31637     ESTABLISHED 39592/haproxy
tcp        0      0 10.242.0.10:3243        141.125.67.34:31637     ESTABLISHED 39592/haproxy
tcp        0      0 10.242.0.10:4625        141.125.67.34:31637     ESTABLISHED 39592/haproxy
tcp        0      0 127.0.0.1:52251         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 127.0.0.1:9098          127.0.0.1:65313         TIME_WAIT   -
tcp        0      0 127.0.0.1:52191         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 172.20.0.1:2040         10.242.0.10:14819       ESTABLISHED 39592/haproxy
tcp        0      0 127.0.0.1:52209         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 127.0.0.1:52195         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 127.0.0.1:52181         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 127.0.0.1:52241         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 10.242.0.10:179         10.242.0.9:43536        ESTABLISHED 37164/bird
tcp        0      0 127.0.0.1:52205         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 10.242.0.10:41457       10.242.0.4:5473         ESTABLISHED 36888/calico-node
tcp        0      0 172.20.0.1:53349        172.20.0.1:2040         ESTABLISHED 6558/kube-proxy
tcp        0      0 127.0.0.1:52237         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 127.0.0.1:9098          127.0.0.1:65269         TIME_WAIT   -
tcp        0      0 127.0.0.1:65315         127.0.0.1:9098          TIME_WAIT   -
tcp        0      0 127.0.0.1:9098          127.0.0.1:65297         TIME_WAIT   -
tcp        0      0 127.0.0.1:52177         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 172.20.0.1:2040         172.20.0.1:53349        ESTABLISHED 39592/haproxy
tcp        0      0 10.242.0.10:179         10.242.0.4:49516        ESTABLISHED 37164/bird
tcp        0      0 10.242.0.10:41447       10.242.0.4:5473         ESTABLISHED 36880/calico-node
tcp        0      0 127.0.0.1:9098          127.0.0.1:65329         TIME_WAIT   -
tcp        0      0 127.0.0.1:52223         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 127.0.0.1:52227         127.0.0.1:9099          TIME_WAIT   -
tcp        0      0 10.242.0.10:41453       10.242.0.4:5473         ESTABLISHED 36881/calico-node
tcp        0   3692 10.242.0.10:22          124.126.137.167:26667   ESTABLISHED 60519/sshd: root@pt
tcp        0      0 10.242.0.10:14819       172.21.0.1:443          ESTABLISHED 36355/calico-typha
tcp        0      0 10.242.0.10:41455       10.242.0.4:5473         ESTABLISHED 36882/calico-node
tcp        0      0 172.20.0.1:53317        172.20.0.1:2040         ESTABLISHED 6500/kubelet
tcp        0      0 10.242.0.10:179         10.242.0.5:5306         ESTABLISHED 37164/bird
tcp        0      0 127.0.0.1:9098          127.0.0.1:65343         TIME_WAIT   -
tcp        0      0 127.0.0.1:9098          127.0.0.1:65265         TIME_WAIT   -
tcp6       0      0 :::22                   :::*                    LISTEN      803/sshd
tcp6       0      0 :::5473                 :::*                    LISTEN      36355/calico-typha
tcp6       0      0 :::9091                 :::*                    LISTEN      36888/calico-node
tcp6       0      0 :::10250                :::*                    LISTEN      6500/kubelet
tcp6       0      0 :::10256                :::*                    LISTEN      6558/kube-proxy
tcp6       0      0 10.242.0.10:10250       10.242.0.5:63528        ESTABLISHED 6500/kubelet
``` 


## Check routes on Worker and PeerPod

- Show all routes:

```
# route -n
Kernel IP routing table
Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
0.0.0.0         10.244.0.1      0.0.0.0         UG    0      0        0 ens3
0.0.0.0         10.244.0.1      0.0.0.0         UG    100    0        0 ens3
10.244.0.0      0.0.0.0         255.255.255.224 U     0      0        0 ens3
10.244.0.1      0.0.0.0         255.255.255.255 UH    100    0        0 ens3
127.0.0.10      0.0.0.0         255.255.255.254 U     0      0        0 vethlocal
172.17.11.192   10.244.0.5      255.255.255.192 UG    0      0        0 ens3
172.17.45.0     10.244.0.4      255.255.255.192 UG    0      0        0 ens3
172.17.52.128   10.244.0.10     255.255.255.192 UG    0      0        0 ens3
172.17.54.0     0.0.0.0         255.255.255.192 U     0      0        0 *
172.17.54.1     0.0.0.0         255.255.255.255 UH    0      0        0 cali3aa8a8ca014
172.17.54.2     0.0.0.0         255.255.255.255 UH    0      0        0 cali76e0604906e
172.17.54.4     0.0.0.0         255.255.255.255 UH    0      0        0 cali245df4d91ba
172.20.0.1      0.0.0.0         255.255.255.255 UH    0      0        0 lo
```

- Get a route detail:

```
# ip route get 172.17.52.128 from 10.244.0.8
172.17.52.128 from 10.244.0.8 via 10.244.0.10 dev ens3 uid 0
    cache
```

## Dump all packets at a network interface

- Check interfaces:

```
# ifconfig
...
ens3      Link encap:Ethernet  HWaddr 02:00:03:33:5D:23
          inet addr:10.244.0.4  Bcast:10.244.0.31  Mask:255.255.255.224
          inet6 addr: fe80::3ff:fe33:5d23/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:1726126 errors:0 dropped:0 overruns:0 frame:0
          TX packets:2162619 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000
          RX bytes:2778926404 (2.5 GiB)  TX bytes:230891724 (220.1 MiB)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:1065973 errors:0 dropped:0 overruns:0 frame:0
          TX packets:1065973 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000
          RX bytes:1874606196 (1.7 GiB)  TX bytes:1874606196 (1.7 GiB)

tunl0     Link encap:UNSPEC  HWaddr 00-00-00-00-00-00-00-00-00-00-00-00-00-00-00-00
          inet addr:172.17.45.0  Mask:255.255.255.255
          UP RUNNING NOARP  MTU:1480  Metric:1
          RX packets:214 errors:0 dropped:0 overruns:0 frame:0
          TX packets:179 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000
          RX bytes:41545 (40.5 KiB)  TX bytes:79562 (77.6 KiB)
```

- Dump packets:

```
# tcpdump -vv -i ens3 |grep 10.244.0.8
tcpdump: listening on ens3, link-type EN10MB (Ethernet), capture size 262144 bytes
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 1, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 1, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 2, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 2, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 3, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 3, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 4, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 4, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 5, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 5, length 64
02:20:26.231408 ARP, Ethernet (len 6), IPv4 (len 4), Request who-has 10.244.0.8 tell pod2pod-zvsi-2, length 28
02:20:26.231579 ARP, Ethernet (len 6), IPv4 (len 4), Reply 10.244.0.8 is-at 02:00:0a:33:5d:23 (oui Unknown), length 46
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 6, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 6, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 7, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 7, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 8, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 8, length 64
    10.244.0.8 > pod2pod-zvsi-2: ICMP echo request, id 22, seq 9, length 64
    pod2pod-zvsi-2 > 10.244.0.8: ICMP echo reply, id 22, seq 9, length 64
    10.244.0.8.44062 > pod2pod-zvsi-2.bgp: Flags [P.], cksum 0x4cee (correct), seq 2628078977:2628078996, ack 4013706653, win 128, options [nop,nop,TS val 515704189 ecr 607843040], length 19: BGP
    pod2pod-zvsi-2.bgp > 10.244.0.8.44062: Flags [.], cksum 0x1620 (incorrect -> 0x75e6), seq 1, ack 19, win 128, options [nop,nop,TS val 607899138 ecr 515704189], length 0
```

## Log dropped packets

- Set iptables to log dropped packstes: 
```bash
iptables -N LOGGING
iptables -A INPUT -j LOGGING
iptables -A OUTPUT -j LOGGING
iptables -A LOGGING -m limit --limit 2/min -j LOG --log-prefix "IPTables-Dropped: " --log-level 4
iptables -A LOGGING -j DROP
```

- Check dropping log:

```
# tail -f /var/log/syslog |grep IPTables-Dropped
Aug 29 08:39:59 pod2pod-zvsi-2 kernel: [365338.445299] IPTables-Dropped: IN= OUT=lo SRC=172.20.0.1 DST=172.20.0.1 LEN=60 TOS=0x00 PREC=0x00 TTL=64 ID=52374 DF PROTO=TCP SPT=10909 DPT=2040 WINDOW=65535 RES=0x00 SYN URGP=0
```
