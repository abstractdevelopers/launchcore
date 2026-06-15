<?php

/**
 * ADD THIS LINE TO: app/Enums/BuildPackTypes.php
 * 
 * Find the enum and add:
 * case LAUNCHPACK = 'launchpack';
 * 
 * Full file should look like:
 * 
 * enum BuildPackTypes: string
 * {
 *     case NIXPACKS = 'nixpacks';
 *     case STATIC = 'static';
 *     case DOCKERFILE = 'dockerfile';
 *     case DOCKERCOMPOSE = 'dockercompose';
 *     case RAILPACK = 'railpack';
 *     case LAUNCHPACK = 'launchpack';  // ADD THIS LINE
 * }
 */
