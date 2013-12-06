zfsziltest
==========

SSD/HDD speed tester/validator

The main purpose of this test is to ensure that a device (possibly an SSD) can be used,
and can be trusted for ZFS intent log.

Original idea: http://brad.livejournal.com/2116715.html

Currently only works on linux.

Usage: zfsziltest.pl <device>

Under linux, specify the device as /dev/disk/by-id/...

Warning: the script needs root privileges, and destroys all data on the given device.
If you dont trust the code, review it, or just dont run it.
The script will issue synchronized writes to the device, at random positions. After 10 seconds,
it will tell you that you can unplug the device. After detaching, you should attach it back,
and if everything works on your system (udev, etc.) then the device will appear with the same
path at /dev, and the script will recognize this, and will validate the written data. It will report
found errors.
In optimal cases, all drive should pass this test, as most HDDs do.

