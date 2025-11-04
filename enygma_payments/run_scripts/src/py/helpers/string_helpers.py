from tempfile import mkstemp
from shutil import move, copymode
import os
#*******************************************************************************
def replace_in_file(filepath, current_text, new_text):
   fd, abspath = mkstemp()
   with os.fdopen(fd,'w') as file1:
       with open(filepath,'r') as file0:
           for line in file0:
               file1.write(line.replace(current_text, new_text))
   copymode(filepath, abspath)
   os.remove(filepath)
   move(abspath, filepath)
#*******************************************************************************

