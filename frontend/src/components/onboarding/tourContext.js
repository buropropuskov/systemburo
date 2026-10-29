/** Read the current stores without changing the application's access policy. */
export function readTourContext(auth, permissions, {
  approvalRole, factTableRoute, hasOwnApplication, userTypeCode, securityPostContext, factRouteLoaded, accessibleContext,
}) {
  return {
    isAuthenticated: auth.isAuthenticated,
    isSecurity: userTypeCode === 'security',
    can: key => permissions.hasPermission(key),
    allowsApplicationAction: key => permissions.allowsApplicationAction(key),
    canViewAccessibleAttachments: userTypeCode === 'security' || auth.isSuperAdmin
      || permissions.hasPermission('page.available'),
    permissionsLoaded: permissions.loaded === true,
    approvalRole: approvalRole.value,
    factTableRoute: factTableRoute.value,
    securityPostContext: securityPostContext.value,
    factRouteLoaded: factRouteLoaded.value,
    accessibleContext: accessibleContext.value,
    hasOwnApplication: hasOwnApplication.value,
  };
}
