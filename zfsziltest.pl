#!/usr/bin/perl

use common::sense;
our $BLOCKSIZE = 4 * 1024;

#require "sys/ioctl.ph";
use Fcntl qw(SEEK_SET);
use Time::HiRes qw(gettimeofday tv_interval);
use Data::Dumper;

$| = 1;

if (@ARGV == 0) {
	print <<EOF;
This script will perform some tests on the given block device.
It will possibly destroy data on it, but will check data reliability
and performance of the drive. Its idea is based on 
http://brad.livejournal.com/2116715.html but keeping the server and client
on the same machine, making the test simpler. Todays desktop boards tend to
have SATA hotswap as well, so you can make the test opening your box, plugging
in the SATA drive (SSD or other), and when the test runs, just simply removing
the power from the drive.
It is important to remove the power, not the SATA cable!

You will be instructed during the test.
EOF
	die;
}

if ($> != 0) {
	die "Please run this script as root!";
}

my ($dev) = @ARGV;
if (!-b $dev) {
	die "Device $dev is not a block device";
}

my $smartctl = `which smartctl`;
my $smart_i;
if ($smartctl eq '') {
	warn("Warning: no smartctl found, drive identity could not be read");
} else {
	$smart_i = `smartctl -i $dev`;
}

print "This will DESTROY DATA on the drive : $dev\n";
if ($smart_i) {
	print "smartctl -i $dev gave:\n$smart_i\n";
}
print "Are you sure? [type yes in uppercase] ";
chomp(my $ans = <STDIN>);
die unless $ans eq 'YES';

our $MAXBLOCK;
my $fh;
my %map;
my $cnt = 0;

sub random_block {
	my ($rnd) = @_;
	return $rnd x ($BLOCKSIZE / length($rnd));
}

{
	my $msg = 10;
	my $blocks;
	my $rnd;
	open($rnd, "-|", "openssl enc -aes-128-ctr -pass pass:123456 -in /dev/zero 2>/dev/null");
	binmode($rnd);
	open($fh, ">", $dev) // die;
	binmode($fh);
	ioctl($fh, 0x00001260, $blocks);
	$blocks = unpack("V4", $blocks);
	$MAXBLOCK = int($blocks * 512 / $BLOCKSIZE);
	my $start = [gettimeofday];
	my $now = [@$start];
	my $next = [$now->[0] + 1, $now->[1]];

	for (;;) {
		my $pos = int(rand($MAXBLOCK));
		my $rnddata;
		sysread($rnd, $rnddata, 16);
		my $data = &random_block($rnddata);

		sysseek($fh, $pos * $BLOCKSIZE, SEEK_SET);

		delete $map{$pos};
		if (syswrite($fh, $data) != $BLOCKSIZE) {
			warn("syswrite: $!");
			last;
		}
		if (!$fh->sync()) {
			warn("fsync: $!");
			last;
		}
		$map{$pos} = $rnddata;
		++$cnt;

		$now = [gettimeofday];
		if (tv_interval($now, $next) <= 0) {
			my $int = tv_interval($start, $now);
			print STDERR "Written $cnt sync blocks, at " . int($cnt / $int) . "/s\r";
			++$next->[0];
			if ($msg > 1) {
				--$msg;
			} elsif ($msg == 1) {
				print STDERR "\nNow you may unplug your device\n";
				--$msg;
			}
		}
	}
}

print STDERR "\nWrite done.\nWaiting for $dev to become available again\n";

for (;;) {
	if (!-b $dev) {
		last;
	}
	select(undef, undef, undef, 0.1);
}

my $fh;

for (;;) {
	if (open($fh, "<", $dev)) {
		last;
	}
	select(undef, undef, undef, 0.1);
}

print STDERR "Verifying device\n";
$cnt = scalar(keys %map);
my $rcnt = 1;

my $errors = 0;

{
	while (my ($pos, $rnddata) = each %map) {
		my $block = &random_block($rnddata);
		my $rblock;
		sysseek($fh, $pos * $BLOCKSIZE, SEEK_SET);
		if (sysread($fh, $rblock, $BLOCKSIZE) != $BLOCKSIZE) {
			die "sysread: $!";
		}
		if ($block ne $rblock) {
			$errors++;
			print STDERR "Block $pos has invalid data\n";
		}
		print STDERR "$rcnt/$cnt\r";
		++$rcnt;
	}
}

print STDERR "\nTotally your device had $errors errors\n";

