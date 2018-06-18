#!/usr/bin/perl 

use Data::Dumper;
use strict;
use warnings;

# key value will be '`schema`.`table`.#'
my %tableCol = ();

# Read the schema file from stdin, from mysqldump --no-data

open my $schema, $ARGV[0] or die "Could not open $ARGV[0]: $!";

    my $inTable=0;
    my $colNum=1;
    my $db='';
    my $table='';

while( my $line = <$schema>)  {

    #print "$line";

    chomp($line);
    if ( $line =~ m/^USE\ \`.*\`\;$/ ) {
        chop($line); # remove ;
        my @t=split(/ /,$line);
        $db = $t[1];
        #print "DB = '$db'\n";
    }

    if ( $line =~ m/^CREATE\ TABLE.*\($/ ) {
        my @t=split(/ /,$line);
        $table=$t[2];
        $inTable=1;
    }
    
    if ( $line =~ m/^\)\ ENGINE\=.*\;$/ ) {
        $inTable=0;
        $colNum=1;
    }

    if ( $inTable == 1 ) {
        #print "in table\n";
        #isCol=$(echo $line | grep -ic '^`.*,$')
        #if ( $line =~ m/^\S*[\x60].*,$/ ) {
        if ( $line =~ m/^\ \ [\x60].*$/ ) {
            my @t= split(/`/,$line);
            #print "col=$t[1]\n";
            $tableCol{$db . "." . $table . "_" . $colNum}=$t[1];
            $colNum=$colNum+1;
            #print $db . "." . $table . "_" . $colNum . "\n";
        }
    }
    
    #print "$db $table $inTable $colNum \n";
    #last if $. == 2;
}

close($schema);
#print Dumper(%tableCol);

my $inUpdate=0;
my $addComma=0;
my $addAnd=0;
my $inUpdWhere=0;
my $WHERE='';
my $out='';
my $ignoreSchema=0;

while( my $line = <stdin>)  {
    #is a database
    
    #print "$line";

    #if ( $printNext == 1 ) {
    #    print "$line";
    #    $printNext=0;
    #}

    chomp($line);

    if ( $line =~ m/^[\x23].*$/ ) {

	if ( $line =~ m/^[\x23].*server\ id\ .*Table_map:\ [\x60]([a-zA-Z0-9_]*)[\x60].*$/ ) {
		if ( $1 eq 'LMS_Work_DB') {
			$ignoreSchema=1;
		} else {
			$ignoreSchema=0;		
		}
	}

	if ( $ignoreSchema != 1 ) {
	        ### ;INSERT INTO `schema`.`table`
        	if ( $line =~ m/^[\x23][\x23][\x23]\ INSERT\ INTO\ .*$/ ) {
	            my @t=split(/ /,$line);
        	    $table=$t[3];
	            print ";INSERT INTO $table\n";
        	}

	        ### ;UPDATE `schema`.`table`
        	if ( $line =~ m/^[\x23][\x23][\x23]\ UPDATE\ .*$/ ) {
	            my @t=split(/ /,$line);
        	    $table=$t[2];
	            print ";UPDATE $table\n";
        	    $inUpdate=1;
	        }
        
        	### ;DELETE FROM `dojo`.`Recus`
        	if ( $line =~ m/^[\x23][\x23][\x23]\ DELETE\ FROM\ .*$/ ) {
            	my @t=split(/ /,$line);
	            $table=$t[3];
        	    print ";DELETE FROM $table\n";
	        }

        	if ( $line =~ m/^[\x23][\x23][\x23]\ SET$/ ) {
	            print "SET\n";
        	    $addComma=2;  #First is not needed
	            $addAnd=0;
        	}

	        if ( $line =~ m/^[\x23][\x23][\x23]\ WHERE$/ ) {
        	    if ( $inUpdate == 1 ) {
                	$WHERE="WHERE ";
	                $inUpdWhere=1;
        	    } else {
                	print "WHERE\n";
	            }
        	    $addAnd=2;    #First is not needed
	            $addComma=0;
        	}

	        if ( $line =~ m/^[\x23][\x23][\x23]\ \ \ \@[1-9].*$/ ) {
        	    my @t=split(/ /,$line);
	            my @t1=split(/=/,$t[3]);
        	    my @t2=split(/@/,$t1[0]);
	            my $col=$t2[1];
        	    #print "key = " . $table . "_" . $col . "\n";
	            #print $tableCol{$table . "_" . $col} . "\n";
		    if (exists $tableCol{$table . "_" . $col}) { 
            		$out=$tableCol{$table . "_" . $col} . '=' . $t1[1];
		    } else {
			print "# Column name not found for key: " . $table . "_" . $col . "\n";
		    }

	            if ( $addComma == 1 ) {
        	        print ",$out\n";
	            } elsif ( $addComma == 2 ) {
        	        print "$out\n";
                	$addComma=1;
	            }
            
        	    if ( $addAnd == 1 ) {
                	if ( $inUpdate == 1 && $inUpdWhere == 1 ) {
                        	$WHERE="$WHERE and $out ";
	                } else {
        	                print "and $out\n";
                	}
	            } elsif ( $addAnd == 2 ) {
        	        if ( $inUpdate == 1 && $inUpdWhere == 1 ) {
                	        $WHERE="$WHERE $out ";
	                } else {
        	                print "$out\n";
                	}
	                $addAnd=1;
        	    }
	        }

        	if ( $line =~ m/^[\x23]\ at\ [0-9]*$/ ) {
	            if ( $inUpdate == 1 ) {
        	        print "$WHERE\n";
                	$inUpdate=0;
	                $inUpdWhere=0;
        	        $WHERE='';
            	    }
	            print ";\n"; 
        	    print "$line\n";
            	    $addComma=0; 
            	    $addAnd=0;
        	}
	} else {
		print "Ignoring schema\n";
	}
    } else {
        print "$line\n";
    }

    # si '/*!*/;' sur la ligne print suivante
    #if ( $line =~ ^.*\/\*\!\*\/\;.*$ ) {
    #    $printNext=1;
    #}

}
