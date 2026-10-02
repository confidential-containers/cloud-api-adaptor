# provisioning-generator

This program is supposed to be used as a [systemd-generator](https://www.freedesktop.org/software/systemd/man/latest/systemd.generator.html) within a PodVM. The generator is running early in systemd's boot process. It's role is to determine the provisioning method for the PodVM depending on its environment (Config Drive or one of the IMDS flavors).

The generator will generate drop-in and mount units that are consumed by the process-user-data service.
