package com.ltv.stat.repository;

import com.ltv.stat.entity.RawOrder;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.time.LocalDate;
import java.util.List;
import java.util.Optional;

public interface RawOrderRepository extends JpaRepository<RawOrder, Long> {
    Optional<RawOrder> findByOrderId(String orderId);
    Optional<RawOrder> findByPlatformCodeAndOrderId(String platformCode, String orderId);

    @Query("SELECT r FROM RawOrder r WHERE (r.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (r.platformCode IS NULL OR r.platformCode = '' OR r.platformCode = 'ALL')))")
    List<RawOrder> findByPlatformCode(@Param("platformCode") String platformCode);

    List<RawOrder> findByRegisterDateEt(LocalDate registerDateEt);
    List<RawOrder> findByRegisterDateEtGreaterThanEqual(LocalDate startDate);
    Optional<RawOrder> findTopByMemberIdAndIsSubsAndRenewTypeOrderByIdAsc(String memberId, Integer isSubs, Integer renewType);
    List<RawOrder> findByLandingPageIdIn(List<String> landingPageIds);
    List<RawOrder> findByLandingPageIdInAndRegisterDateEtGreaterThanEqual(List<String> landingPageIds, LocalDate startDate);

    @Query("SELECT r FROM RawOrder r WHERE (r.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (r.platformCode IS NULL OR r.platformCode = '' OR r.platformCode = 'ALL'))) AND r.landingPageId IN :landingPageIds")
    List<RawOrder> findByPlatformCodeAndLandingPageIdIn(@Param("platformCode") String platformCode, @Param("landingPageIds") List<String> landingPageIds);

    List<RawOrder> findByMemberId(String memberId);

    Optional<RawOrder> findFirstByPlatformCodeAndMemberIdOrderByPayTimeBjAsc(String platformCode, String memberId);

    List<RawOrder> findByPlatformCodeAndMemberIdIn(String platformCode, List<String> memberIds);

    @Query("SELECT DISTINCT r.landingPageId FROM RawOrder r WHERE (r.platformCode = :platformCode OR (:platformCode = 'rocnovel' AND (r.platformCode IS NULL OR r.platformCode = '' OR r.platformCode = 'ALL'))) AND r.landingPageId IS NOT NULL AND r.landingPageId != ''")
    List<String> findDistinctLandingPageIdsByPlatformCode(@Param("platformCode") String platformCode);
}
